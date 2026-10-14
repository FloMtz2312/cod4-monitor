package api

import (
	"database/sql"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/evasion"
	"cod4monitor/internal/store"

	"github.com/gin-gonic/gin"
)

// The moderation inbox gathers what the staff has to look at, newest first:
//   report    a !report not handled yet (30 days)
//   suspect   abnormal stats over 7 days: headshot rate far above the other players', or a huge K/D
//   watch     a player tagged "suspect" / "surveiller" in the staff notes is playing now
//   evasion   possible ban evasion (internal/evasion), and the same from B3's database (b3evasion)
//   b3ban     a ban or temp ban given by B3 or an admin in game (7 days)
// Reports are closed like before (handled_by); the other items are dismissed for the whole team (inbox_dismissed).
// Only the staff with the private data sees the items built from it (evasions, B3); reports are enough for !report.

const (
	suspectMinKills = 20  // over 7 days
	suspectHS       = 0.6 // headshot rate, and at least twice the server's
	suspectKD       = 6.0 // with at least 30 kills
)

type inboxItem map[string]any

func inboxRoutes(g *gin.RouterGroup, st *store.Store) {
	db := st.DB
	g.GET("/inbox", func(c *gin.Context) {
		id := mustID(c)
		now := time.Now()
		private := role(c) != roleVisitor // owner, or the "private" permission
		items, b3on, err := inbox(db, st, id, now, private)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		dismissed, err := dismissedItems(db, id)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		out := []inboxItem{}
		for _, it := range items {
			if dismissed[it["id"].(string)] {
				continue
			}
			// the demos of the player around the facts (private data: the recording of the player's game)
			if key, _ := it["key"].(string); private && key != "" {
				ts, _ := it["ts"].(int64)
				if ds, err := st.Demos(id, store.DemoFilter{Player: key, From: ts - 30*60, To: ts + 10*60, Limit: 10}); err == nil && len(ds) > 0 {
					it["demos"] = ds
				}
			}
			out = append(out, it)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i]["ts"].(int64) > out[j]["ts"].(int64) })
		c.JSON(200, gin.H{"items": out, "b3": b3on, "private": private})
	})

	// Classify an item for the whole team: a report is marked handled, the others are hidden from the inbox.
	g.POST("/inbox/dismiss", sameOrigin, func(c *gin.Context) {
		var req struct{ ID string }
		if c.ShouldBindJSON(&req) != nil || req.ID == "" || len(req.ID) > 300 {
			c.JSON(400, gin.H{"error": "élément manquant"})
			return
		}
		id, user, now := mustID(c), c.GetString("user"), time.Now().Unix()
		c.Set("audit", req.ID)
		var err error
		if rid, ok := strings.CutPrefix(req.ID, "report:"); ok {
			_, err = db.Exec(`UPDATE reports SET handled_by=?, handled_at=? WHERE id=? AND server_id=? AND handled_at IS NULL`, user, now, rid, id)
		} else {
			_, err = db.Exec(`INSERT INTO inbox_dismissed VALUES(?,?,?,?) ON CONFLICT(server_id, item) DO NOTHING`, id, req.ID, user, now)
		}
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
}

// dismissedItems: the items the team classified (reports are closed in their own table).
func dismissedItems(db *sql.DB, id int64) (map[string]bool, error) {
	rs, err := rows(db, `SELECT item FROM inbox_dismissed WHERE server_id=?`, id)
	if err != nil {
		return nil, err
	}
	out := map[string]bool{}
	for _, r := range rs {
		out[r["item"].(string)] = true
	}
	return out, nil
}

func inbox(db *sql.DB, st *store.Store, id int64, now time.Time, private bool) ([]inboxItem, bool, error) {
	var out []inboxItem
	// reports
	reps, err := rows(db, `SELECT id, ts, reporter_key, reporter_name, target, reason FROM reports
		WHERE server_id=? AND handled_at IS NULL AND ts>=? ORDER BY id DESC LIMIT 100`, id, now.AddDate(0, 0, -30).Unix())
	if err != nil {
		return nil, false, err
	}
	for _, r := range reps {
		it := inboxItem{"id": fmt.Sprintf("report:%d", r["id"]), "type": "report", "ts": r["ts"], "reporter_name": r["reporter_name"],
			"reporter_key": r["reporter_key"], "target": r["target"], "reason": r["reason"]}
		// the reported player: latest name containing what was typed
		var key, name string
		if t, _ := r["target"].(string); t != "" && db.QueryRow(`SELECT guid, name_raw FROM player_names WHERE server_id=? AND name_clean LIKE '%'||?||'%'
			ORDER BY last_seen DESC LIMIT 1`, id, t).Scan(&key, &name) == nil {
			it["key"], it["name_raw"] = key, name
		}
		out = append(out, it)
	}
	if !private {
		return out, false, nil
	}

	// abnormal stats of the last 7 days
	since := now.AddDate(0, 0, -7).Unix()
	_, week := now.ISOWeek()
	ss, err := rows(db, `WITH k AS (SELECT * FROM kills WHERE server_id=?1 AND ts>=?2),
		s AS (SELECT killer_key AS key, COUNT(*) AS kills, SUM(cause='MOD_HEAD_SHOT') AS hs, MAX(ts) AS last FROM k
			WHERE killer_key IS NOT NULL AND teamkill=0 GROUP BY killer_key),
		d AS (SELECT victim_key AS key, COUNT(*) AS deaths FROM k GROUP BY victim_key)
		SELECT s.key, s.kills, COALESCE(d.deaths, 0) AS deaths, s.hs, s.last,
			(SELECT SUM(kills) FROM s) AS all_kills, (SELECT SUM(hs) FROM s) AS all_hs,
			(SELECT killer_name FROM k k2 WHERE k2.killer_key=s.key ORDER BY ts DESC LIMIT 1) AS name_raw
		FROM s LEFT JOIN d ON d.key=s.key WHERE s.kills>=?3`, id, since, suspectMinKills)
	if err != nil {
		return nil, false, err
	}
	for _, s := range ss {
		kills, deaths, hs := s["kills"].(int64), s["deaths"].(int64), s["hs"].(int64)
		// headshot rate of the other players (the suspect would raise their own reference)
		otherKills, otherHS := s["all_kills"].(int64)-kills, s["all_hs"].(int64)-hs
		serverHS := 0.0
		if otherKills > 0 {
			serverHS = float64(otherHS) / float64(otherKills)
		}
		rate, kd := float64(hs)/float64(kills), float64(kills)/float64(max(deaths, 1))
		var why []string
		if rate >= suspectHS && rate >= 2*serverHS {
			why = append(why, "hs")
		}
		if kd >= suspectKD && kills >= 30 {
			why = append(why, "kd")
		}
		if len(why) == 0 {
			continue
		}
		out = append(out, inboxItem{"id": fmt.Sprintf("suspect:%s:%d", s["key"], week), "type": "suspect", "ts": s["last"], "key": s["key"],
			"name_raw": s["name_raw"], "kills": kills, "deaths": deaths, "headshots": hs, "hs_rate": rate, "server_hs_rate": serverHS, "why": why})
	}

	// tagged players in game now (status of the last poll)
	ws, err := rows(db, `WITH live AS (SELECT ps.name_clean, ps.name_raw FROM player_sightings ps
			WHERE ps.snapshot_id=(SELECT MAX(id) FROM snapshots WHERE server_id=?1 AND online=1)
			AND (SELECT online FROM snapshots WHERE server_id=?1 ORDER BY id DESC LIMIT 1)=1),
		m AS (SELECT l.name_raw, COALESCE((SELECT guid FROM player_names pn WHERE pn.server_id=?1 AND pn.name_clean=l.name_clean
			ORDER BY last_seen DESC LIMIT 1), 'n:'||l.name_clean) AS key FROM live l)
		SELECT m.key, m.name_raw, n.tag, n.note FROM m JOIN player_notes n ON n.server_id=?1 AND n.key=m.key
		WHERE n.tag IN ('suspect', 'surveiller')`, id)
	if err != nil {
		return nil, false, err
	}
	day := now.Format("2006-01-02")
	for _, w := range ws {
		out = append(out, inboxItem{"id": fmt.Sprintf("watch:%s:%s", w["key"], day), "type": "watch", "ts": now.Unix(), "key": w["key"],
			"name_raw": w["name_raw"], "tag": w["tag"], "note": w["note"]})
	}

	// possible ban evasion, active this week
	ev, err := evasion.Find(db, id, since)
	if err != nil {
		return nil, false, err
	}
	seen := map[string]bool{}
	for _, e := range ev {
		if e.Score < evasion.Alert {
			continue
		}
		seen[e.Key] = true
		out = append(out, inboxItem{"id": "evasion:" + e.Key + ":" + e.BanKey, "type": "evasion", "ts": e.LastSeen, "key": e.Key,
			"name_raw": e.NameRaw, "ban_key": e.BanKey, "ban_name": e.BanName, "ban_ts": e.BanTS, "score": e.Score, "reasons": e.Reasons})
	}

	// B3: its bans of the week and the evasions it sees
	conf, found, err := st.ServerConf(id)
	if err != nil || !found || conf.Logs.B3DB == "" {
		return out, false, err
	}
	d, err := b3.Open(conf.Logs.B3DB)
	if err != nil {
		return out, true, nil // B3 unreachable: the rest of the inbox still shows
	}
	defer d.Close()
	for _, typ := range []string{b3.Ban, b3.TempBan} {
		ps, err := d.Penalties(b3.Filter{Type: typ, Limit: 50}, now)
		if err != nil {
			return out, true, nil
		}
		for _, p := range ps {
			if p.TimeAdd < since {
				continue
			}
			out = append(out, inboxItem{"id": "b3ban:" + strconv.FormatInt(p.ID, 10), "type": "b3ban", "ts": p.TimeAdd, "key": p.GUID,
				"name_raw": p.Name, "pid": p.ID, "ban_type": p.Type, "admin": p.Admin, "origin": p.Origin, "reason": p.Reason,
				"keyword": p.Keyword, "expire": p.Expire, "active": p.Active})
		}
	}
	es, err := d.Evasions(now)
	if err != nil {
		return out, true, nil
	}
	for _, e := range es {
		if seen[e.Suspect.GUID] {
			continue
		}
		out = append(out, inboxItem{"id": fmt.Sprintf("b3evasion:%d:%d", e.Suspect.ID, e.Banned.ID), "type": "evasion", "ts": e.Suspect.TimeEdit,
			"key": e.Suspect.GUID, "name_raw": e.Suspect.Name, "ban_key": e.Banned.GUID, "ban_name": e.Banned.Name, "ban_ts": e.BanAt,
			"reason": e.Reason, "shared_ips": len(e.Shared), "source": "b3"})
	}
	return out, true, nil
}
