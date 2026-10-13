package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

var (
	reasonBad  = regexp.MustCompile(`["';%/\\]`) // characters CoD4X refuses in a ban/kick reason
	durRe      = regexp.MustCompile(`^\d{1,4}[mhd]$`)
	banIDRe    = regexp.MustCompile(`^[\w:\[\]]+$`)
	dvarRe     = regexp.MustCompile(`is: "([^"]*)"`) // rcon "<dvar>" -> "xp_scale" is: "1^7" default: ...
	dvarNameRe = regexp.MustCompile(`^\w{1,64}$`)
)

// adminRoutes: rcon console, kick, ban/unban. Every command sent is kept in rcon_log.
func adminRoutes(g *gin.RouterGroup, st *store.Store, rcons RconOf) {
	// exec runs and audits one command. ok=false means the response was already written.
	exec := func(c *gin.Context, cmd string, audit bool) (out string, ok bool) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		run := rcons.of(id)
		if run == nil {
			c.JSON(404, gin.H{"error": "rcon non configuré pour ce serveur"})
			return "", false
		}
		out, err := run(cmd)
		out = query.RedactSecrets(out)
		var ne net.Error
		noReply := errors.As(err, &ne) && ne.Timeout()
		if audit {
			slog.Info("rcon", "server_id", id, "command", query.RedactSecrets(cmd))
			logErr := err
			if noReply {
				logErr = errors.New("pas de réponse")
			}
			if err := st.LogRcon(id, time.Now(), cmd, out, logErr); err != nil {
				slog.Error("rcon log", "err", err)
			}
		}
		if noReply { // e.g. "map": the server is busy loading and doesn't answer
			return "", true
		}
		if err != nil {
			c.JSON(502, gin.H{"error": err.Error()})
			return "", false
		}
		return out, true
	}

	// online finds the player's current slot with a fresh "status" (slots change between polls).
	online := func(c *gin.Context, name string) (query.RconPlayer, bool) {
		out, ok := exec(c, "status", false)
		if !ok {
			return query.RconPlayer{}, false
		}
		var match []query.RconPlayer
		for _, p := range query.ParseRconStatus(out) {
			if query.StripColors(p.Name) == query.StripColors(name) {
				match = append(match, p)
			}
		}
		if len(match) != 1 {
			c.JSON(409, gin.H{"error": fmt.Sprintf("%d joueur(s) nommé(s) %q sur le serveur", len(match), query.StripColors(name))})
			return query.RconPlayer{}, false
		}
		return match[0], true
	}

	g.POST("/rcon", sameOrigin, func(c *gin.Context) {
		var req struct {
			Command string `json:"command"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Command) == "" {
			c.JSON(400, gin.H{"error": "commande manquante"})
			return
		}
		if !permittedCommand(st.DB, c, strings.TrimSpace(req.Command)) {
			c.JSON(403, gin.H{"error": "commande non autorisée"})
			return
		}
		c.Set("audit", strings.TrimSpace(req.Command))
		if out, ok := exec(c, strings.TrimSpace(req.Command), true); ok {
			c.JSON(200, gin.H{"output": out, "no_reply": out == ""})
		}
	})

	g.POST("/kick", sameOrigin, func(c *gin.Context) {
		var req struct {
			Name string `json:"name"` // raw name, with colors
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
			c.JSON(400, gin.H{"error": "joueur manquant"})
			return
		}
		p, ok := online(c, req.Name)
		if !ok {
			return
		}
		cmd := fmt.Sprintf("clientkick %d", p.Num)
		c.Set("audit", query.StripColors(req.Name))
		if out, ok := exec(c, cmd, true); ok {
			c.JSON(200, gin.H{"output": out, "command": cmd})
		}
	})

	// Private message to an online player (rcon tell): the text cannot carry a second command.
	g.POST("/tell", sameOrigin, func(c *gin.Context) {
		var req struct{ Name, Message string }
		msg := ""
		if c.ShouldBindJSON(&req) == nil {
			msg = strings.TrimSpace(strings.Map(func(r rune) rune {
				if r == '"' || r == ';' || r == '\\' || r < 0x20 {
					return -1
				}
				return r
			}, req.Message))
		}
		if req.Name == "" || msg == "" {
			c.JSON(400, gin.H{"error": "joueur ou message manquant"})
			return
		}
		msg = string([]rune(msg)[:min(len([]rune(msg)), 200)])
		p, ok := online(c, req.Name)
		if !ok {
			return
		}
		c.Set("audit", query.StripColors(req.Name)+" : "+msg)
		if out, ok := exec(c, fmt.Sprintf(`tell %d "%s"`, p.Num, msg), true); ok {
			c.JSON(200, gin.H{"output": out})
		}
	})

	// Ban an online player: duration "" = permanent, else 30m / 12h / 7d.
	g.POST("/ban", sameOrigin, func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		var req struct{ Name, Duration, Reason string }
		if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" || (req.Duration != "" && !durRe.MatchString(req.Duration)) {
			c.JSON(400, gin.H{"error": "joueur ou durée invalide (ex : 30m, 12h, 7d, vide = permanent)"})
			return
		}
		reason := strings.TrimSpace(reasonBad.ReplaceAllString(req.Reason, ""))
		if reason == "" {
			reason = "Banni par un admin"
		}
		reason = reason[:min(len(reason), 126)]
		p, ok := online(c, req.Name)
		if !ok {
			return
		}
		cmd := fmt.Sprintf("permban %d %s", p.Num, reason)
		if req.Duration != "" {
			cmd = fmt.Sprintf("tempban %d %s %s", p.Num, req.Duration, reason)
		}
		dur := req.Duration
		if dur == "" {
			dur = "permanent"
		}
		c.Set("audit", fmt.Sprintf("%s (%s) : %s", query.StripColors(req.Name), dur, reason))
		out, ok := exec(c, cmd, true)
		if !ok {
			return
		}
		ip := query.HostOf(p.Addr)
		if err := st.AddBan(id, time.Now(), p.GUID, p.Name, ip, req.Duration, reason, cmd); err != nil {
			slog.Error("store ban", "err", err)
		}
		c.JSON(200, gin.H{"output": out, "command": cmd})
	})

	g.POST("/unban", sameOrigin, func(c *gin.Context) {
		var req struct{ ID string }
		if err := c.ShouldBindJSON(&req); err != nil || !banIDRe.MatchString(req.ID) {
			c.JSON(400, gin.H{"error": "identifiant invalide"})
			return
		}
		cmd := "unban " + req.ID
		c.Set("audit", req.ID)
		if out, ok := exec(c, cmd, true); ok {
			c.JSON(200, gin.H{"output": out, "command": cmd})
		}
	})

	// Active bans as the server lists them, plus the bans issued from this page.
	g.GET("/bans", func(c *gin.Context) {
		var active string
		if rcons.of(mustID(c)) != nil {
			out, ok := exec(c, "dumpbanlist", false)
			if !ok {
				return
			}
			active = out
		}
		issued, err := rows(st.DB, `SELECT ts, guid, name_raw, ip, duration, reason FROM bans WHERE server_id=? ORDER BY ts DESC LIMIT 200`, c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"active": active, "issued": issued})
	})

	// Read one server dvar (xp_scale, scr_mapvote_maps...): "xp_scale" is: "1^7" default: ...
	g.GET("/dvar", func(c *gin.Context) {
		name := c.Query("name")
		if !dvarNameRe.MatchString(name) {
			c.JSON(400, gin.H{"error": "nom de variable invalide"})
			return
		}
		r, ok := exec(c, name, false)
		if !ok {
			return
		}
		m := dvarRe.FindStringSubmatch(r)
		if m == nil {
			c.JSON(404, gin.H{"error": "variable inconnue"})
			return
		}
		c.JSON(200, gin.H{"name": name, "value": strings.TrimSuffix(m[1], "^7")})
	})

	g.GET("/rcon/log", func(c *gin.Context) {
		r, err := rows(st.DB, `SELECT ts, command, output, error FROM rcon_log WHERE server_id=? ORDER BY id DESC LIMIT 200`, c.Param("id"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"log": r})
	})
}

func mustID(c *gin.Context) int64 {
	id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
	return id
}
