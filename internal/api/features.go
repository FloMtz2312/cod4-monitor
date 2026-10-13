package api

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/store"
)

var atTimeRe = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

// featureRoutes: records, duels, admin notes, scheduled rcon commands.
func featureRoutes(g *gin.RouterGroup, st *store.Store, rcons RconOf) {
	db := st.DB
	fail := func(c *gin.Context, err error) bool {
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return err != nil
	}

	// Records of the period: best kill streak (reset by a death or a new round), most kills / headshots in a round
	// and in a match (consecutive rounds on the same map), plus the players of the day and of the week.
	g.GET("/records", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		ks, err := rows(db, `SELECT k.ts, k.round_id, k.killer_key, k.killer_name, k.victim_key, k.teamkill, k.cause, r.map
			FROM kills k LEFT JOIN rounds r ON r.id=k.round_id WHERE k.server_id=? AND k.ts>=? AND k.ts<? ORDER BY k.ts, k.id`, id, from, to)
		if fail(c, err) {
			return
		}
		rounds, err := rows(db, `SELECT id, map FROM rounds WHERE server_id=? AND started_at>=? AND started_at<? ORDER BY started_at`, id, from-86400, to)
		if fail(c, err) {
			return
		}
		match := map[int64]int64{} // round id -> first round id of its match
		var cur, prevMap any
		for _, r := range rounds {
			if cur == nil || r["map"] != prevMap {
				cur = r["id"]
			}
			match[r["id"].(int64)], prevMap = cur.(int64), r["map"]
		}
		type rec struct {
			Key, Name, Map string
			Value          int
			TS             int64
		}
		best := map[string]*rec{}
		keep := func(kind, key, name, mp string, v int, ts int64) {
			if b := best[kind]; b == nil || v > b.Value {
				best[kind] = &rec{key, name, mp, v, ts}
			}
		}
		streak, streakRound := map[string]int{}, map[string]any{}
		type cnt struct{ kills, hs int }
		perRound, perMatch := map[string]*cnt{}, map[string]*cnt{}
		for _, k := range ks {
			ts := k["ts"].(int64)
			mp, _ := k["map"].(string)
			if v, _ := k["victim_key"].(string); v != "" {
				streak[v] = 0
			}
			kk, _ := k["killer_key"].(string)
			if kk == "" || k["teamkill"].(int64) == 1 {
				continue
			}
			name, _ := k["killer_name"].(string)
			if streakRound[kk] != k["round_id"] { // S&D: no respawn, a streak can't cross rounds
				streak[kk], streakRound[kk] = 0, k["round_id"]
			}
			streak[kk]++
			keep("streak", kk, name, mp, streak[kk], ts)
			hs := 0
			if k["cause"] == "MOD_HEAD_SHOT" {
				hs = 1
			}
			if rid, ok := k["round_id"].(int64); ok {
				for kind, m := range map[string]map[string]*cnt{"round": perRound, "match": perMatch} {
					grp := rid
					if kind == "match" {
						grp = match[rid]
					}
					gk := kk + "|" + strconv.FormatInt(grp, 10)
					if m[gk] == nil {
						m[gk] = &cnt{}
					}
					m[gk].kills += 1
					m[gk].hs += hs
					keep(kind+"_kills", kk, name, mp, m[gk].kills, ts)
					keep(kind+"_headshots", kk, name, mp, m[gk].hs, ts)
				}
			}
		}
		top := func(since int64) (map[string]any, error) {
			r, err := rows(db, `SELECT killer_key AS key, (SELECT killer_name FROM kills k2 WHERE k2.server_id=?1 AND k2.killer_key=k.killer_key ORDER BY ts DESC LIMIT 1) AS name_raw,
				COUNT(*) AS kills, SUM(cause='MOD_HEAD_SHOT') AS headshots FROM kills k
				WHERE server_id=?1 AND ts>=?2 AND killer_key IS NOT NULL AND teamkill=0 GROUP BY killer_key ORDER BY kills DESC LIMIT 1`, id, since)
			if err != nil || len(r) == 0 {
				return nil, err
			}
			return r[0], nil
		}
		now := time.Now().Unix()
		day, err := top(now - 86400)
		if fail(c, err) {
			return
		}
		week, err := top(now - 7*86400)
		if fail(c, err) {
			return
		}
		// longest kills (distance from the mod's KE line, meters)
		for kind, cond := range map[string]string{"longest": "", "longest_noscope": ` AND ','||k.tags||',' LIKE '%,ns,%'`} {
			var r rec
			if db.QueryRow(`SELECT k.killer_key, k.killer_name, COALESCE(rd.map, ''), CAST(ROUND(k.distance) AS INTEGER), k.ts FROM kills k
				LEFT JOIN rounds rd ON rd.id=k.round_id WHERE k.server_id=? AND k.ts>=? AND k.ts<? AND k.teamkill=0 AND k.distance IS NOT NULL`+cond+`
				ORDER BY k.distance DESC LIMIT 1`, id, from, to).Scan(&r.Key, &r.Name, &r.Map, &r.Value, &r.TS) == nil {
				best[kind] = &r
			}
		}
		c.JSON(200, gin.H{"records": best, "day": day, "week": week})
	})

	// Head to head between two players (keys: GUID or n:<name>).
	g.GET("/duel", func(c *gin.Context) {
		id, a, b := c.Param("id"), c.Query("a"), c.Query("b")
		if a == "" || b == "" || a == b {
			c.JSON(400, gin.H{"error": "choisis deux joueurs différents"})
			return
		}
		side := func(k, v string) (any, error) {
			return rows(db, `SELECT COUNT(*) AS kills, SUM(cause='MOD_HEAD_SHOT') AS headshots, MAX(ts) AS last,
				(SELECT weapon FROM kills k2 WHERE k2.server_id=?1 AND k2.killer_key=?2 AND k2.victim_key=?3 GROUP BY weapon ORDER BY COUNT(*) DESC LIMIT 1) AS weapon,
				(SELECT killer_name FROM kills k3 WHERE k3.server_id=?1 AND k3.killer_key=?2 ORDER BY ts DESC LIMIT 1) AS name_raw
				FROM kills WHERE server_id=?1 AND killer_key=?2 AND victim_key=?3`, id, k, v)
		}
		ra, err := side(a, b)
		if fail(c, err) {
			return
		}
		rb, err := side(b, a)
		if fail(c, err) {
			return
		}
		c.JSON(200, gin.H{"a": ra, "b": rb})
	})

	g.GET("/notes", func(c *gin.Context) {
		// names: every name of the GUID, so the tag shows whatever name the player uses
		r, err := rows(db, `SELECT key, tag, note, (SELECT group_concat(name_clean, '|') FROM player_names p
			WHERE p.server_id=n.server_id AND p.guid=n.key) AS names FROM player_notes n WHERE server_id=?`, c.Param("id"))
		if !fail(c, err) {
			c.JSON(200, gin.H{"notes": r})
		}
	})

	g.PUT("/notes", sameOrigin, func(c *gin.Context) {
		var req struct{ Key, Tag, Note string }
		if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" || len(req.Tag) > 32 || len(req.Note) > 2000 {
			c.JSON(400, gin.H{"error": "note invalide"})
			return
		}
		who := strings.TrimPrefix(req.Key, "n:") // audit detail: the player's latest name rather than a GUID
		db.QueryRow(`SELECT name_clean FROM player_names WHERE server_id=? AND guid=? ORDER BY last_seen DESC LIMIT 1`, c.Param("id"), req.Key).Scan(&who)
		c.Set("audit", strings.TrimSpace(who+" "+req.Tag))
		var err error
		if req.Tag == "" && strings.TrimSpace(req.Note) == "" {
			_, err = db.Exec(`DELETE FROM player_notes WHERE server_id=? AND key=?`, c.Param("id"), req.Key)
		} else {
			_, err = db.Exec(`INSERT INTO player_notes VALUES(?,?,?,?,?) ON CONFLICT(server_id, key) DO UPDATE SET
				tag=excluded.tag, note=excluded.note, updated_at=excluded.updated_at`, c.Param("id"), req.Key, req.Tag, req.Note, time.Now().Unix())
		}
		if !fail(c, err) {
			c.JSON(200, gin.H{"ok": true})
		}
	})

	g.GET("/schedules", func(c *gin.Context) {
		r, err := rows(db, `SELECT id, command, every_min, at_time, enabled, last_run FROM schedules WHERE server_id=? ORDER BY id`, c.Param("id"))
		if !fail(c, err) {
			c.JSON(200, gin.H{"schedules": r})
		}
	})

	// every_min (repeat) or at "HH:MM" (once a day, local time)
	g.POST("/schedules", sameOrigin, func(c *gin.Context) {
		var req struct {
			Command  string
			EveryMin int `json:"every_min"`
			At       string
		}
		if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Command) == "" || strings.ContainsAny(req.Command, "\r\n") ||
			(req.EveryMin > 0) == (req.At != "") || (req.At != "" && !atTimeRe.MatchString(req.At)) || req.EveryMin < 0 || req.EveryMin > 10080 {
			c.JSON(400, gin.H{"error": "commande + « toutes les N minutes » OU « à HH:MM »"})
			return
		}
		c.Set("audit", strings.TrimSpace(req.Command))
		var every, at any
		if req.EveryMin > 0 {
			every = req.EveryMin
		} else {
			at = req.At
		}
		// last_run = now: a repeated command waits one period before its first run
		_, err := db.Exec(`INSERT INTO schedules(server_id, command, every_min, at_time, last_run) VALUES(?,?,?,?,?)`,
			c.Param("id"), strings.TrimSpace(req.Command), every, at, time.Now().Unix())
		if !fail(c, err) {
			c.JSON(200, gin.H{"ok": true})
		}
	})

	scheduleCmd := func(c *gin.Context) { // audit detail: the command, before it is deleted
		var cmd string
		db.QueryRow(`SELECT command FROM schedules WHERE id=? AND server_id=?`, c.Param("sid"), c.Param("id")).Scan(&cmd)
		c.Set("audit", cmd)
	}
	g.POST("/schedules/:sid/toggle", sameOrigin, func(c *gin.Context) {
		scheduleCmd(c)
		_, err := db.Exec(`UPDATE schedules SET enabled=1-enabled WHERE id=? AND server_id=?`, c.Param("sid"), c.Param("id"))
		if !fail(c, err) {
			c.JSON(200, gin.H{"ok": true})
		}
	})

	g.DELETE("/schedules/:sid", sameOrigin, func(c *gin.Context) {
		scheduleCmd(c)
		_, err := db.Exec(`DELETE FROM schedules WHERE id=? AND server_id=?`, c.Param("sid"), c.Param("id"))
		if !fail(c, err) {
			c.JSON(200, gin.H{"ok": true})
		}
	})
}

// RunSchedules sends the due scheduled commands every 30 s, until ctx is cancelled.
func RunSchedules(ctx context.Context, st *store.Store, rcons RconOf) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(30 * time.Second):
		}
		due, err := rows(st.DB, `SELECT id, server_id, command, every_min, at_time, last_run FROM schedules WHERE enabled=1`)
		if err != nil {
			slog.Error("schedules", "err", err)
			continue
		}
		now := time.Now()
		for _, s := range due {
			last, _ := s["last_run"].(int64)
			ok := false
			if every, isEvery := s["every_min"].(int64); isEvery {
				ok = now.Unix()-last >= every*60
			} else if at, _ := s["at_time"].(string); at != "" {
				hm, _ := time.ParseInLocation("15:04", at, time.Local)
				today := time.Date(now.Year(), now.Month(), now.Day(), hm.Hour(), hm.Minute(), 0, 0, time.Local)
				ok = !now.Before(today) && last < today.Unix() && now.Sub(today) < 10*time.Minute // missed by >10 min: skip today
			}
			sid, srv := s["id"].(int64), s["server_id"].(int64)
			run := rcons.of(srv)
			if !ok || run == nil {
				continue
			}
			cmd := s["command"].(string)
			out, err := run(cmd)
			slog.Info("scheduled rcon", "server_id", srv, "command", cmd, "err", err)
			if err := st.LogRcon(srv, now, "[programmé] "+cmd, out, err); err != nil {
				slog.Error("rcon log", "err", err)
			}
			if _, err := st.DB.Exec(`UPDATE schedules SET last_run=? WHERE id=?`, now.Unix(), sid); err != nil {
				slog.Error("schedules", "err", err)
			}
		}
	}
}
