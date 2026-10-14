package api

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/push"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Push notifications of the staff: each browser subscribes with what it wants to receive (Notifications screen of
// the v2 app). Server down or back is sent by internal/watch; the moderation items come from RunPushScan below.

// Access tells, outside of a request, whether an account owns a server, else what it may do there.
func Access(db *sql.DB, adminUser string, noauth bool) func(user string, id int64) (bool, map[string]bool) {
	adminUser = strings.ToLower(strings.TrimSpace(adminUser))
	return func(user string, id int64) (bool, map[string]bool) {
		if noauth || user == adminUser {
			return true, nil
		}
		var acct, owner string
		if db.QueryRow(`SELECT account_role FROM users WHERE username=?`, user).Scan(&acct) != nil {
			return false, nil // account deleted
		}
		if acct == "admin" && db.QueryRow(`SELECT owner FROM servers WHERE id=?`, id).Scan(&owner) == nil && owner != "" && owner == user {
			return true, nil
		}
		return false, grants(db, user, id)
	}
}

func pushRoutes(r *gin.Engine, st *store.Store, ps *push.Sender) {
	// the key the browser subscribes with, and the choices of this browser if it is subscribed with this account
	r.GET("/api/push", func(c *gin.Context) {
		out := gin.H{"key": ps.PublicKey(), "kinds": push.Kinds}
		if e := c.Query("endpoint"); e != "" {
			if sub, ok, err := st.PushSub(e); err == nil && ok && sub.Username == c.GetString("user") {
				out["sub"] = sub
			}
		}
		c.JSON(200, out)
	})

	r.PUT("/api/push", sameOrigin, func(c *gin.Context) {
		if ps == nil {
			c.JSON(503, gin.H{"error": "notifications indisponibles sur ce monitor"})
			return
		}
		var in struct {
			Endpoint string `json:"endpoint"`
			Keys     struct {
				P256dh string `json:"p256dh"`
				Auth   string `json:"auth"`
			} `json:"keys"`
			Lang  string   `json:"lang"`
			Types []string `json:"types"`
			Muted []int64  `json:"muted"`
		}
		if c.ShouldBindJSON(&in) != nil || !push.ValidEndpoint(in.Endpoint) || len(in.Muted) > 200 {
			c.JSON(400, gin.H{"error": "abonnement invalide"})
			return
		}
		pub, err1 := b64url(in.Keys.P256dh)
		auth, err2 := b64url(in.Keys.Auth)
		if err1 != nil || err2 != nil || len(pub) != 65 || len(auth) != 16 {
			c.JSON(400, gin.H{"error": "clés du navigateur invalides"})
			return
		}
		types := []string{}
		for _, k := range in.Types {
			if !slices.Contains(push.Kinds, k) {
				c.JSON(400, gin.H{"error": "type de notification inconnu"})
				return
			}
			if !slices.Contains(types, k) {
				types = append(types, k)
			}
		}
		if in.Lang != "en" && in.Lang != "de" {
			in.Lang = "fr"
		}
		sub := store.PushSub{Endpoint: in.Endpoint, Username: c.GetString("user"), P256dh: pushB64.EncodeToString(pub), Auth: pushB64.EncodeToString(auth),
			Lang: in.Lang, Types: types, Muted: in.Muted}
		if err := st.SavePushSub(sub, time.Now()); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"sub": sub})
	})

	r.DELETE("/api/push", sameOrigin, func(c *gin.Context) {
		var in struct{ Endpoint string }
		if c.ShouldBindJSON(&in) != nil || in.Endpoint == "" {
			c.Status(400)
			return
		}
		if err := st.DeletePushSub(in.Endpoint, c.GetString("user")); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})

	// a test notification to this browser: shows at once whether the whole chain works
	r.POST("/api/push/test", sameOrigin, func(c *gin.Context) {
		var in struct{ Endpoint string }
		c.ShouldBindJSON(&in)
		sub, ok, err := st.PushSub(in.Endpoint)
		if ps == nil || err != nil || !ok || sub.Username != c.GetString("user") {
			c.JSON(404, gin.H{"error": "ce navigateur n'est pas abonné"})
			return
		}
		err = ps.Send(sub, push.Message{Kind: "test", Title: "Notifications activées", TitleEN: "Notifications are on",
			Body: "Les alertes choisies arriveront ici, même app fermée.", BodyEN: "The alerts you chose will show up here, even with the app closed.",
			Path: "notifications", Tag: "test"})
		if errors.Is(err, push.ErrGone) {
			c.JSON(410, gin.H{"error": "le service de notifications ne connaît plus ce navigateur : réactive les notifications"})
			return
		}
		if err != nil {
			c.JSON(502, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
}

var pushB64 = base64.RawURLEncoding

func b64url(s string) ([]byte, error) {
	return pushB64.DecodeString(strings.TrimRight(s, "="))
}

// RunPushScan announces by push each new item of the moderation inbox of the servers: a !report, abnormal stats, a
// tagged player in game, a possible ban evasion, a B3 ban. A live event (report, ban, connection) has its server
// scanned a few seconds later; every server is also scanned each minute. Items are announced once (push_sent); the
// first scan of a server only records what is already there.
func RunPushScan(ctx context.Context, st *store.Store, hub *Hub, ps *push.Sender) {
	events := hub.sub()
	defer hub.unsub(events)
	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	scan := func(ids ...int64) {
		names := map[int64]string{}
		confs, err := st.ServerConfs()
		if err != nil {
			slog.Error("push scan", "err", err)
			return
		}
		for _, c := range confs {
			if c.Enabled && (len(ids) == 0 || slices.Contains(ids, c.ID)) {
				names[c.ID] = query.StripColors(c.Name)
			}
		}
		for id, name := range names {
			if err := pushScan(st, ps, id, name, time.Now()); err != nil {
				slog.Warn("push scan", "server", name, "err", err)
			}
		}
	}
	scan()
	var pending []int64
	var soon <-chan time.Time
	purged := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			scan()
			if time.Since(purged) > 24*time.Hour {
				st.PurgePushSent(time.Now().AddDate(0, 0, -45))
				purged = time.Now()
			}
		case b := <-events:
			var ev struct {
				Type     string `json:"type"`
				ServerID int64  `json:"server_id"`
			}
			if json.Unmarshal(b, &ev) == nil && ev.ServerID > 0 && (ev.Type == "report" || ev.Type == "b3ban" || ev.Type == "connection" || ev.Type == "profile") {
				if !slices.Contains(pending, ev.ServerID) {
					pending = append(pending, ev.ServerID)
				}
				if soon == nil {
					soon = time.After(3 * time.Second) // the log batch is committed meanwhile
				}
			}
		case <-soon:
			scan(pending...)
			pending, soon = nil, nil
		}
	}
}

func pushScan(st *store.Store, ps *push.Sender, id int64, server string, now time.Time) error {
	items, _, err := inbox(st.DB, st, id, now, true)
	if err != nil {
		return err
	}
	dismissed, err := dismissedItems(st.DB, id)
	if err != nil {
		return err
	}
	seeded := "push_seeded_" + strconv.FormatInt(id, 10)
	first := st.Meta(seeded) == ""
	for _, it := range items {
		iid, _ := it["id"].(string)
		if dismissed[iid] {
			continue
		}
		fresh, err := st.PushSent(id, iid, now)
		if err != nil {
			return err
		}
		ts, _ := it["ts"].(int64)
		if !fresh || first || ts < now.Add(-24*time.Hour).Unix() { // old news (the monitor was off): only in the inbox
			continue
		}
		if m, ok := inboxPush(it, server); ok {
			m.Server = id
			ps.Notify(m)
		}
	}
	if first {
		return st.SetMeta(seeded, "1")
	}
	return nil
}

// inboxPush: the notification of an inbox item.
func inboxPush(it inboxItem, server string) (push.Message, bool) {
	str := func(k string) string { s, _ := it[k].(string); return strings.TrimSpace(query.StripColors(s)) }
	name := str("name_raw")
	m := push.Message{Kind: "suspect", Need: []string{"private"}, Path: "moderation", Tag: fmt.Sprint(it["id"])}
	switch it["type"] {
	case "report":
		target, reporter, reason := str("target"), str("reporter_name"), str("reason")
		m.Kind, m.Need, m.Urgent = "report", []string{"reports", "private"}, true
		m.Title, m.TitleEN = "Signalement : "+target, "Report: "+target
		if reason != "" {
			m.Body, m.BodyEN = fmt.Sprintf("%s : « %s » · %s", reporter, reason, server), fmt.Sprintf("%s: \"%s\" · %s", reporter, reason, server)
		} else {
			m.Body, m.BodyEN = "Signalé par "+reporter+" · "+server, "Reported by "+reporter+" · "+server
		}
	case "suspect":
		why, _ := it["why"].([]string)
		var fr, en []string
		if slices.Contains(why, "hs") {
			rate, _ := it["hs_rate"].(float64)
			others, _ := it["server_hs_rate"].(float64)
			fr = append(fr, fmt.Sprintf("%.0f %% de headshots (les autres : %.0f %%)", rate*100, others*100))
			en = append(en, fmt.Sprintf("%.0f%% headshots (others: %.0f%%)", rate*100, others*100))
		}
		if slices.Contains(why, "kd") {
			kills, _ := it["kills"].(int64)
			deaths, _ := it["deaths"].(int64)
			kd := fmt.Sprintf("K/D %.1f", float64(kills)/float64(max(deaths, 1)))
			fr, en = append(fr, kd), append(en, kd)
		}
		m.Title, m.TitleEN = "Stats anormales : "+name, "Abnormal stats: "+name
		m.Body, m.BodyEN = strings.Join(fr, " · ")+" sur 7 jours · "+server, strings.Join(en, " · ")+" over 7 days · "+server
	case "watch":
		tag, note := str("tag"), str("note")
		fr, en := "Noté « Suspect »", "Tagged \"Suspect\""
		if tag == "surveiller" {
			fr, en = "Noté « À surveiller »", "Tagged \"To watch\""
		}
		if note != "" {
			fr, en = fr+" : "+note, en+": "+note
		}
		m.Title, m.TitleEN = name+" est en jeu", name+" is playing"
		m.Body, m.BodyEN = fr+" · "+server, en+" · "+server
	case "evasion":
		ban := str("ban_name")
		m.Title, m.TitleEN = "Contournement de ban possible", "Possible ban evasion"
		m.Body, m.BodyEN = fmt.Sprintf("%s ressemble à %s, banni · %s", name, ban, server), fmt.Sprintf("%s looks like %s, banned · %s", name, ban, server)
	case "b3ban":
		admin, reason := str("admin"), str("reason")
		if admin == "" {
			admin = "B3"
		}
		m.Kind = "b3ban"
		m.Title, m.TitleEN = name+" banni", name+" banned"
		if it["ban_type"] == b3.TempBan {
			m.Title, m.TitleEN = name+" banni temporairement", name+" temp-banned"
		}
		m.Body, m.BodyEN = "Par "+admin, "By "+admin
		if reason != "" {
			m.Body, m.BodyEN = m.Body+" : "+reason, m.BodyEN+": "+reason
		}
		m.Body, m.BodyEN = m.Body+" · "+server, m.BodyEN+" · "+server
	default:
		return m, false
	}
	return m, true
}
