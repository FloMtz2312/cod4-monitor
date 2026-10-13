package api

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// b3Routes: the penalties, admins and moderation stats of the server's B3 admin bot, read from its database, and
// the actions taken from the dashboard (warn, kick, ban, temp ban, unban, group). The penalties are written in B3's
// database, so B3 enforces them and keeps their history; a player online is also told / kicked at once by rcon.
func b3Routes(g *gin.RouterGroup, st *store.Store, rcons RconOf) {
	// open the server's B3 database; ok=false means the response was already written
	open := func(c *gin.Context) (*b3.DB, bool) {
		conf, found, err := st.ServerConf(mustID(c))
		switch {
		case err != nil:
			c.JSON(500, gin.H{"error": err.Error()})
		case !found || conf.Logs.B3DB == "":
			c.JSON(404, gin.H{"error": "B3 n'est pas configuré pour ce serveur (Admin → serveur → réglages avancés)", "b3": false})
		default:
			d, err := b3.Open(conf.Logs.B3DB)
			if err != nil {
				c.JSON(502, gin.H{"error": "base B3 : " + err.Error()})
				return nil, false
			}
			return d, true
		}
		return nil, false
	}
	fail := func(c *gin.Context, err error) bool {
		if err != nil {
			code := 500
			if errors.Is(err, b3.ErrNoClient) || errors.Is(err, b3.ErrNoGroup) {
				code = 404
			}
			c.JSON(code, gin.H{"error": err.Error()})
		}
		return err != nil
	}
	// player online: slot and name from a fresh rcon status (found=false if offline or no rcon)
	online := func(c *gin.Context, guid string) (p query.RconPlayer, run func(string) (string, error), found bool) {
		run = rcons.of(mustID(c))
		if run == nil {
			return p, nil, false
		}
		out, err := run("status")
		if err != nil {
			return p, run, false
		}
		for _, x := range query.ParseRconStatus(out) {
			if x.GUID == guid {
				return x, run, true
			}
		}
		return p, run, false
	}
	logRcon := func(c *gin.Context, run func(string) (string, error), cmd string) string {
		out, err := run(cmd)
		if err := st.LogRcon(mustID(c), time.Now(), cmd, out, err); err != nil {
			slog.Error("rcon log", "err", err)
		}
		return out
	}

	g.GET("/b3", func(c *gin.Context) {
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		groups, err := d.Groups()
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"b3": true, "groups": groups})
	})

	g.GET("/b3/penalties", func(c *gin.Context) {
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		limit, _ := strconv.Atoi(c.Query("limit"))
		ps, err := d.Penalties(b3.Filter{Type: c.Query("type"), Active: c.Query("active") == "1", Query: c.Query("q"), GUID: c.Query("guid"), Limit: limit}, time.Now())
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"penalties": ps})
	})

	g.GET("/b3/admins", func(c *gin.Context) {
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		as, err := d.Admins()
		if fail(c, err) {
			return
		}
		groups, err := d.Groups()
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"admins": as, "groups": groups})
	})

	g.GET("/b3/stats", func(c *gin.Context) {
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		days, _ := strconv.Atoi(c.Query("days"))
		if days <= 0 || days > 730 {
			days = 90
		}
		now := time.Now()
		s, err := d.Stats(now.AddDate(0, 0, -days), now)
		if fail(c, err) {
			return
		}
		c.JSON(200, s)
	})

	// One player: B3 group, connections and penalties.
	g.GET("/b3/player", func(c *gin.Context) {
		guid := c.Query("guid")
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		cl, found, err := d.Client(guid)
		if fail(c, err) {
			return
		}
		ps, err := d.Penalties(b3.Filter{GUID: guid, Limit: 200}, time.Now())
		if fail(c, err) {
			return
		}
		groups, err := d.Groups()
		if fail(c, err) {
			return
		}
		res := gin.H{"known": found, "penalties": ps, "groups": groups}
		if found {
			res["client"] = cl
			names, err := d.Names(cl.ID)
			if fail(c, err) {
				return
			}
			ips, err := d.IPs(cl.ID)
			if fail(c, err) {
				return
			}
			linked, err := d.Linked(cl.ID)
			if fail(c, err) {
				return
			}
			res["names"], res["ips"], res["linked"] = names, ips, linked
		}
		c.JSON(200, res)
	})

	// Players by name (current or past), IP or GUID, like B3's !lookup.
	g.GET("/b3/search", func(c *gin.Context) {
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		cs, err := d.Search(c.Query("q"), 100)
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"clients": cs})
	})

	// Players not banned who connected from the IP of a banned player after the ban.
	g.GET("/b3/evasions", func(c *gin.Context) {
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		es, err := d.Evasions(time.Now())
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"evasions": es})
	})

	// Warn, kick, temp ban or ban a player.
	g.POST("/b3/penalty", sameOrigin, func(c *gin.Context) {
		var req struct {
			GUID    string `json:"guid"`
			Name    string `json:"name"` // raw, with colors
			Type    string `json:"type"` // Warning, Kick, TempBan, Ban
			Minutes int64  `json:"minutes"`
			Reason  string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GUID == "" {
			c.JSON(400, gin.H{"error": "joueur manquant"})
			return
		}
		reason := strings.TrimSpace(reasonBad.ReplaceAllString(req.Reason, ""))
		if reason == "" {
			c.JSON(400, gin.H{"error": "indique une raison"})
			return
		}
		reason = reason[:min(len(reason), 120)]
		if (req.Type == b3.TempBan || req.Type == b3.Warning) && (req.Minutes <= 0 || req.Minutes > 365*24*60) {
			c.JSON(400, gin.H{"error": "durée invalide"})
			return
		}
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		p, run, on := online(c, req.GUID)
		name := query.StripColors(req.Name)
		if on {
			name = query.StripColors(p.Name)
		}
		ip := ""
		if on {
			ip = query.HostOf(p.Addr)
		}
		id, err := d.Add(req.Type, req.GUID, name, ip, req.Minutes, reason, c.GetString("user"), time.Now())
		if fail(c, err) {
			return
		}
		label := map[string]string{b3.Warning: "avertissement", b3.Kick: "kick", b3.TempBan: "ban temporaire", b3.Ban: "ban"}[req.Type]
		detail := fmt.Sprintf("%s : %s — %s", name, label, reason)
		if req.Type == b3.TempBan || req.Type == b3.Warning {
			detail += fmt.Sprintf(" (%s)", minutesStr(req.Minutes))
		}
		c.Set("audit", detail)
		res := gin.H{"id": id, "online": on}
		if on {
			switch req.Type {
			case b3.Warning:
				logRcon(c, run, fmt.Sprintf("tell %d ^1Avertissement^7 : %s", p.Num, reason))
			default: // B3 also kicks banned players within 10 s; this is immediate
				logRcon(c, run, fmt.Sprintf("clientkick %d %s", p.Num, reason))
			}
		}
		c.JSON(200, res)
	})

	// Lift all the bans and temp bans of a player.
	g.POST("/b3/unban", sameOrigin, func(c *gin.Context) {
		var req struct {
			GUID string `json:"guid"`
			Name string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || !banIDRe.MatchString(req.GUID) {
			c.JSON(400, gin.H{"error": "joueur manquant"})
			return
		}
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		n, err := d.Unban(req.GUID, time.Now())
		if fail(c, err) {
			return
		}
		// B3 also bans on the game server itself when it re-applies a ban: lift that one too
		if run := rcons.of(mustID(c)); run != nil {
			logRcon(c, run, "unban "+req.GUID)
		}
		c.Set("audit", fmt.Sprintf("%s : %d ban(s) levé(s)", query.StripColors(req.Name), n))
		c.JSON(200, gin.H{"lifted": n})
	})

	// Lift one penalty (a warning, or one ban).
	g.POST("/b3/penalty/:pid/lift", sameOrigin, func(c *gin.Context) {
		pid, err := strconv.ParseInt(c.Param("pid"), 10, 64)
		if err != nil {
			c.JSON(400, gin.H{"error": "sanction invalide"})
			return
		}
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		if fail(c, d.Deactivate(pid, time.Now())) {
			return
		}
		c.Set("audit", "sanction B3 n°"+c.Param("pid"))
		c.JSON(200, gin.H{"ok": true})
	})

	// Put a player in a B3 group (level 0 guest … 100 super admin). Applies at their next connection.
	g.POST("/b3/group", sameOrigin, func(c *gin.Context) {
		var req struct {
			GUID  string `json:"guid"`
			Name  string `json:"name"`
			Group int64  `json:"group"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.GUID == "" {
			c.JSON(400, gin.H{"error": "joueur manquant"})
			return
		}
		d, ok := open(c)
		if !ok {
			return
		}
		defer d.Close()
		gr, err := d.SetGroup(req.GUID, req.Group, time.Now())
		if fail(c, err) {
			return
		}
		_, _, on := online(c, req.GUID)
		c.Set("audit", fmt.Sprintf("%s → %s (%d)", query.StripColors(req.Name), gr.Name, gr.Level))
		c.JSON(200, gin.H{"group": gr, "online": on})
	})
}

func minutesStr(m int64) string {
	switch {
	case m%(24*60) == 0:
		return fmt.Sprintf("%d j", m/(24*60))
	case m%60 == 0:
		return fmt.Sprintf("%d h", m/60)
	}
	return fmt.Sprintf("%d min", m)
}
