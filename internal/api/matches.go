package api

import (
	"database/sql"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/evasion"
)

// A match is a run of consecutive rounds on the same map and mode (S&D logs an InitGame per round);
// a pause of more than 10 minutes also starts a new one. Its id is the id of its first round.
type match struct {
	ID         int64   `json:"id"`
	Start      int64   `json:"started_at"`
	End        int64   `json:"ended_at"`
	Map        string  `json:"map"`
	Gametype   string  `json:"gametype"`
	Rounds     []int64 `json:"-"`
	RoundCount int     `json:"rounds"`
	Kills      int64   `json:"kills"`
}

const matchGap = 600

// openRound: how long a round without an end line still counts as being played (a crashed server never ends it).
const openRound = 3 * 3600

// matchGroups returns the matches whose first round starts in [from, to), oldest first. Rounds without kills
// (empty server) still join a match, matches without any kill are dropped.
func matchGroups(db *sql.DB, id string, from, to int64) ([]*match, error) {
	rs, err := rows(db, `SELECT r.id, r.started_at, r.ended_at, COALESCE(r.map, '') AS map, COALESCE(r.gametype, '') AS gametype,
		(SELECT COUNT(*) FROM kills k WHERE k.round_id=r.id) AS kills, (SELECT MAX(ts) FROM kills k WHERE k.round_id=r.id) AS last_kill
		FROM rounds r WHERE r.server_id=? AND r.started_at>=? AND r.started_at<? ORDER BY r.started_at, r.id`, id, from-86400, to)
	if err != nil {
		return nil, err
	}
	var all []*match
	var cur *match
	for _, r := range rs {
		start := r["started_at"].(int64)
		end := start
		if v, ok := r["ended_at"].(int64); ok {
			end = v
		} else if v, ok := r["last_kill"].(int64); ok {
			end = v
		}
		mp, gt := r["map"].(string), r["gametype"].(string)
		if cur == nil || mp != cur.Map || gt != cur.Gametype || start-cur.End > matchGap {
			cur = &match{ID: r["id"].(int64), Start: start, Map: mp, Gametype: gt}
			all = append(all, cur)
		}
		cur.End = max(cur.End, end)
		cur.Rounds = append(cur.Rounds, r["id"].(int64))
		cur.RoundCount++
		cur.Kills += r["kills"].(int64)
	}
	out := all[:0]
	for _, m := range all {
		if m.Kills > 0 && m.Start >= from {
			out = append(out, m)
		}
	}
	return out, nil
}

// placeholders returns "?,?,?" and the ids as arguments.
func placeholders(ids []int64) (string, []any) {
	args := make([]any, len(ids))
	for i, v := range ids {
		args[i] = v
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// resolveKey: a name key ("n:<name>") with a known GUID becomes that GUID (all the player's names merged).
func resolveKey(db *sql.DB, id, key string) string {
	if name, ok := strings.CutPrefix(key, "n:"); ok {
		var guid string
		if db.QueryRow(`SELECT guid FROM player_names WHERE server_id=? AND name_clean=? ORDER BY last_seen DESC LIMIT 1`, id, name).Scan(&guid) == nil {
			return guid
		}
	}
	return key
}

func matchRoutes(g *gin.RouterGroup, db *sql.DB) {
	fail := func(c *gin.Context, err error) bool {
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return err != nil
	}

	// Matches of the period, newest first, with their player count and top killer.
	// ?key=: only the matches of this player, with their kills, deaths, team (of their last round) and result
	// (without the player count and top killer).
	g.GET("/matches", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "7d")
		key := c.Query("key")
		if key != "" {
			key = resolveKey(db, id, key)
		}
		ms, err := matchGroups(db, id, from, to)
		if fail(c, err) {
			return
		}
		type agg struct {
			players map[string]bool
			kills   map[string]int
			names   map[string]string
			pk, pd  int    // the player of ?key=
			team    string // their team in the last round they played
			teamTS  int64
		}
		byRound := map[int64]*agg{}
		for _, m := range ms {
			a := &agg{players: map[string]bool{}, kills: map[string]int{}, names: map[string]string{}}
			for _, r := range m.Rounds {
				byRound[r] = a
			}
		}
		q := `SELECT round_id, ts, killer_key, killer_name, killer_team, victim_key, victim_team, teamkill FROM kills
			WHERE server_id=? AND ts>=? AND round_id IS NOT NULL`
		args := []any{id, from}
		if key != "" { // only the player's kills and deaths (indexed): the list shows their figures, not the top killer
			q += ` AND (killer_key=? OR victim_key=?)`
			args = append(args, key, key)
		}
		ks, err := rows(db, q, args...)
		if fail(c, err) {
			return
		}
		for _, k := range ks {
			a := byRound[k["round_id"].(int64)]
			if a == nil {
				continue
			}
			a.players[k["victim_key"].(string)] = true
			if key != "" {
				team, _ := k["victim_team"].(string)
				if k["victim_key"] == key {
					a.pd++
				} else if k["killer_key"] == key {
					team, _ = k["killer_team"].(string)
					if k["teamkill"].(int64) == 0 {
						a.pk++
					}
				} else {
					team = ""
				}
				if ts := k["ts"].(int64); team != "" && ts >= a.teamTS {
					a.team, a.teamTS = team, ts
				}
			}
			if kk, ok := k["killer_key"].(string); ok {
				a.players[kk] = true
				if k["teamkill"].(int64) == 0 {
					a.kills[kk]++
					a.names[kk], _ = k["killer_name"].(string)
				}
			}
		}
		out := []gin.H{}
		for i := len(ms) - 1; i >= 0 && len(out) < 200; i-- {
			m := ms[i]
			a := byRound[m.ID]
			if key != "" && !a.players[key] {
				continue
			}
			top, n := "", 0
			for k, v := range a.kills {
				if v > n || v == n && k < top {
					top, n = k, v
				}
			}
			w, sa, sx := matchResult(db, m)
			item := gin.H{"match": m, "players": len(a.players), "top_key": top, "top_name": a.names[top], "top_kills": n,
				"winner": w, "score_allies": sa, "score_axis": sx}
			if key != "" {
				item = gin.H{"match": m, "winner": w, "score_allies": sa, "score_axis": sx}
				result := ""
				if (w == "allies" || w == "axis") && a.team != "" {
					result = map[bool]string{true: "win", false: "loss"}[w == a.team]
				}
				item["player"] = gin.H{"kills": a.pk, "deaths": a.pd, "team": a.team, "result": result}
			}
			out = append(out, item)
		}
		c.JSON(200, gin.H{"matches": out})
	})

	// Scoreboard of one match (id = any of its rounds).
	g.GET("/match", func(c *gin.Context) {
		id := c.Param("id")
		var start int64
		if db.QueryRow(`SELECT started_at FROM rounds WHERE id=? AND server_id=?`, c.Query("id"), id).Scan(&start) != nil {
			c.JSON(404, gin.H{"error": "match introuvable"})
			return
		}
		ms, err := matchGroups(db, id, start-86400, start+86400)
		if fail(c, err) {
			return
		}
		var m *match
		for _, x := range ms {
			for _, r := range x.Rounds {
				if r == mustInt(c.Query("id")) {
					m = x
				}
			}
		}
		if m == nil {
			c.JSON(404, gin.H{"error": "match introuvable"})
			return
		}
		in, args := placeholders(m.Rounds)
		args = append([]any{id}, args...)
		players, err := rows(db, `WITH k AS (SELECT * FROM kills WHERE server_id=? AND round_id IN (`+in+`)),
			e AS (SELECT killer_key AS key, killer_name AS name, ts, 1-teamkill AS kill, 0 AS death, cause='MOD_HEAD_SHOT' AS hs, teamkill AS tk, 0 AS sui,
					IIF(teamkill=0, ','||COALESCE(tags, '')||',', NULL) AS tg, IIF(teamkill=0, distance, NULL) AS dist
				FROM k WHERE killer_key IS NOT NULL
				UNION ALL SELECT victim_key, victim_name, ts, 0, 1, 0, 0, killer_key IS NULL, NULL, NULL FROM k)
			SELECT key, (SELECT name FROM e e2 WHERE e2.key=e.key ORDER BY ts DESC LIMIT 1) AS name_raw,
				SUM(kill) AS kills, SUM(death) AS deaths, SUM(hs) AS headshots, SUM(tk) AS teamkills, SUM(sui) AS suicides,
				SUM(tg LIKE '%,ns,%') AS noscopes, SUM(tg LIKE '%,qs,%') AS quickscopes, SUM(tg LIKE '%,wb,%') AS wallbangs, MAX(dist) AS longest
			FROM e GROUP BY key ORDER BY kills DESC, deaths`, args...)
		if fail(c, err) {
			return
		}
		acc, err := rows(db, `SELECT key, SUM(shots) AS shots, SUM(hits) AS hits, SUM(dmg_done) AS dmg_done FROM accuracy
			WHERE server_id=? AND round_id IN (`+in+`) GROUP BY key`, args...)
		if fail(c, err) {
			return
		}
		// official end-of-match line of the mod (score = points, level, prestige)
		official, err := rows(db, `SELECT key, team, score, level, prestige FROM match_scores WHERE server_id=? AND round_id IN (`+in+`)`, args...)
		if fail(c, err) {
			return
		}
		// team of each player in the last round they played (when the mod's end-of-match line is missing)
		teams, err := rows(db, `WITH t AS (SELECT killer_key AS key, killer_team AS team, ts, id FROM kills WHERE server_id=? AND round_id IN (`+in+`)
				AND killer_key IS NOT NULL AND killer_team IN ('allies', 'axis')
			UNION ALL SELECT victim_key, victim_team, ts, id FROM kills WHERE server_id=? AND round_id IN (`+in+`) AND victim_team IN ('allies', 'axis'))
			SELECT key, team FROM (SELECT key, team, ROW_NUMBER() OVER (PARTITION BY key ORDER BY ts DESC, id DESC) AS n FROM t) WHERE n=1`,
			append(append([]any{}, args...), args...)...)
		if fail(c, err) {
			return
		}
		teamOf := map[string]any{}
		for _, t := range teams {
			teamOf[t["key"].(string)] = t["team"]
		}
		byKey := map[string]map[string]any{}
		for _, a := range acc {
			byKey[a["key"].(string)] = a
		}
		offKey := map[string]map[string]any{}
		for _, o := range official {
			offKey[o["key"].(string)] = o
		}
		for _, p := range players {
			if a := byKey[p["key"].(string)]; a != nil {
				p["shots"], p["hits"], p["dmg_done"] = a["shots"], a["hits"], a["dmg_done"]
			}
			p["team"] = teamOf[p["key"].(string)]
			if o := offKey[p["key"].(string)]; o != nil {
				p["score"], p["level"], p["prestige"] = o["score"], o["level"], o["prestige"]
				if o["team"] != nil && o["team"] != "" {
					p["team"] = o["team"]
				}
			}
		}
		rounds, err := rows(db, `SELECT r.id, r.started_at, r.ended_at, COUNT(k.id) AS kills, r.winner, r.score_allies, r.score_axis,
			(SELECT killer_name FROM kills k3 WHERE k3.round_id=r.id AND killer_key IS NOT NULL AND teamkill=0 ORDER BY id LIMIT 1) AS first_blood,
			(SELECT group_concat(type||':'||name_raw||':'||COALESCE(value, 0), '|') FROM game_events g WHERE g.round_id=r.id) AS events,
			(SELECT killer_name FROM kills k2 WHERE k2.round_id=r.id AND killer_key IS NOT NULL AND teamkill=0 GROUP BY killer_key ORDER BY COUNT(*) DESC LIMIT 1) AS top_killer
			FROM rounds r LEFT JOIN kills k ON k.round_id=r.id WHERE r.id IN (`+in+`) GROUP BY r.id ORDER BY r.started_at`, args[1:]...)
		if fail(c, err) {
			return
		}
		w, sa, sx := matchResult(db, m)
		timeline, err := matchTimeline(db, id, m.Rounds)
		if fail(c, err) {
			return
		}
		truncated := len(timeline) > 5000
		if truncated {
			timeline = timeline[:5000]
		}
		c.JSON(200, gin.H{"match": m, "players": players, "rounds": rounds, "winner": w, "score_allies": sa, "score_axis": sx, "timeline": timeline, "timeline_truncated": truncated})
	})

	// Badges of the period: the best player of each category (with a minimum of data where a ratio is involved).
	g.GET("/badges", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		now := time.Now().Unix()
		const killsQ = `WITH k AS (SELECT * FROM kills WHERE server_id=?1 AND ts>=?2 AND ts<?3),
			s AS (SELECT key, SUM(kill) AS kills, SUM(death) AS deaths, SUM(hs) AS hs FROM (
				SELECT killer_key AS key, 1-teamkill AS kill, 0 AS death, cause='MOD_HEAD_SHOT' AS hs FROM k WHERE killer_key IS NOT NULL
				UNION ALL SELECT victim_key, 0, 1, 0 FROM k) GROUP BY key) `
		const nameOf = `(SELECT killer_name FROM kills k2 WHERE k2.server_id=?1 AND k2.killer_key=x.key ORDER BY ts DESC LIMIT 1)`
		// connections when enterleave.log is read, else nothing for the activity badges
		const playQ = `WITH c AS (SELECT COALESCE(guid, 'n:'||name_clean) AS key, name_raw, joined_at,
				MAX(0, MIN(COALESCE(left_at, IIF(open=1, ?4, joined_at)), ?3) - MAX(joined_at, ?2)) AS secs
			FROM connections WHERE server_id=?1 AND joined_at<?3 AND IIF(open=1, ?4, COALESCE(left_at, joined_at))>=?2) `
		badges := []struct{ code, q string }{
			{"killer", killsQ + `SELECT key, ` + nameOf + ` AS name_raw, kills AS value FROM s x WHERE kills>0 ORDER BY kills DESC LIMIT 1`},
			{"kd", killsQ + `SELECT key, ` + nameOf + ` AS name_raw, ROUND(1.0*kills/MAX(deaths, 1), 2) AS value FROM s x WHERE kills>=20 ORDER BY value DESC LIMIT 1`},
			{"headshot", killsQ + `SELECT key, ` + nameOf + ` AS name_raw, ROUND(100.0*hs/kills) AS value FROM s x WHERE kills>=20 ORDER BY value DESC LIMIT 1`},
			{"accuracy", `SELECT key, (SELECT name_raw FROM accuracy a2 WHERE a2.server_id=?1 AND a2.key=x.key ORDER BY ts DESC LIMIT 1) AS name_raw,
				ROUND(100.0*SUM(hits)/SUM(shots)) AS value FROM accuracy x WHERE server_id=?1 AND ts>=?2 AND ts<?3
				GROUP BY key HAVING SUM(shots)>=50 ORDER BY value DESC LIMIT 1`},
			{"active", playQ + `SELECT key, (SELECT name_raw FROM c c2 WHERE c2.key=c.key ORDER BY joined_at DESC LIMIT 1) AS name_raw, SUM(secs) AS value
				FROM c GROUP BY key HAVING value>0 ORDER BY value DESC LIMIT 1`},
			{"loyal", playQ + `SELECT key, (SELECT name_raw FROM c c2 WHERE c2.key=c.key ORDER BY joined_at DESC LIMIT 1) AS name_raw,
				COUNT(DISTINCT date(joined_at, 'unixepoch', 'localtime')) AS value FROM c GROUP BY key ORDER BY value DESC LIMIT 1`},
			{"noscope", tagBadge("ns")}, {"quickscope", tagBadge("qs")}, {"wallbang", tagBadge("wb")},
			{"longest", `SELECT killer_key AS key, killer_name AS name_raw, ROUND(distance) AS value FROM kills
				WHERE server_id=?1 AND ts>=?2 AND ts<?3 AND teamkill=0 AND distance IS NOT NULL ORDER BY distance DESC LIMIT 1`},
			{"clutch", `SELECT key, (SELECT name_raw FROM game_events g2 WHERE g2.server_id=?1 AND g2.key=x.key ORDER BY ts DESC LIMIT 1) AS name_raw,
				COUNT(*) AS value FROM game_events x WHERE server_id=?1 AND ts>=?2 AND ts<?3 AND type='clutch' GROUP BY key ORDER BY value DESC LIMIT 1`},
			{"winner", `WITH k AS (SELECT * FROM kills WHERE server_id=?1 AND ts>=?2 AND ts<?3), ` + sniperCTEs + `
				SELECT key, ` + nameOf + ` AS name_raw, ROUND(100.0*won/played) AS value FROM w x WHERE played>=10 ORDER BY value DESC, played DESC LIMIT 1`},
			{"chatty", `SELECT COALESCE(guid, 'n:'||name_clean) AS key, MAX(name_raw) AS name_raw, COUNT(*) AS value FROM chat_messages
				WHERE server_id=?1 AND ts>=?2 AND ts<?3 GROUP BY key ORDER BY value DESC LIMIT 1`},
		}
		out := []gin.H{}
		for _, b := range badges {
			r, err := rows(db, b.q, id, from, to, now)
			if fail(c, err) {
				return
			}
			if len(r) == 1 && r[0]["name_raw"] != nil {
				out = append(out, gin.H{"badge": b.code, "key": r[0]["key"], "name_raw": r[0]["name_raw"], "value": r[0]["value"]})
			}
		}
		c.JSON(200, gin.H{"badges": out})
	})

	// Weekly progression of a player over the last 12 weeks: kills, deaths, headshots, shots and hits.
	g.GET("/progress", func(c *gin.Context) {
		id := c.Param("id")
		key := resolveKey(db, id, c.Query("key"))
		since := time.Now().AddDate(0, 0, -7*12).Unix()
		const week = `strftime('%Y-%W', ts, 'unixepoch', 'localtime')`
		k, err := rows(db, `SELECT `+week+` AS week, MIN(ts) AS ts,
			SUM(killer_key=?2 AND teamkill=0) AS kills, SUM(victim_key=?2) AS deaths, SUM(killer_key=?2 AND cause='MOD_HEAD_SHOT') AS headshots
			FROM kills WHERE server_id=?1 AND ts>=?3 AND (killer_key=?2 OR victim_key=?2) GROUP BY week`, id, key, since)
		if fail(c, err) {
			return
		}
		a, err := rows(db, `SELECT `+week+` AS week, MIN(ts) AS ts, SUM(shots) AS shots, SUM(hits) AS hits
			FROM accuracy WHERE server_id=?1 AND key=?2 AND ts>=?3 GROUP BY week`, id, key, since)
		if fail(c, err) {
			return
		}
		weeks := map[string]map[string]any{}
		for _, r := range append(k, a...) {
			w := r["week"].(string)
			if weeks[w] == nil {
				weeks[w] = map[string]any{"week": w, "ts": r["ts"]}
			}
			for f, v := range r {
				if f != "week" && f != "ts" {
					weeks[w][f] = v
				}
			}
			if r["ts"].(int64) < weeks[w]["ts"].(int64) {
				weeks[w]["ts"] = r["ts"]
			}
		}
		out := []map[string]any{}
		for _, w := range weeks {
			out = append(out, w)
		}
		sort.Slice(out, func(i, j int) bool { return out[i]["week"].(string) < out[j]["week"].(string) })
		c.JSON(200, gin.H{"weeks": out})
	})

	// Per map: matches, length, players, and how many left during a match (needs enterleave.log).
	g.GET("/mapstats", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		ms, err := matchGroups(db, id, from, to)
		if fail(c, err) {
			return
		}
		now := time.Now().Unix()
		cs, err := rows(db, `SELECT joined_at AS a, IIF(open=1, ?1, left_at) AS b, open FROM connections
			WHERE server_id=?2 AND (open=1 OR left_at IS NOT NULL) AND joined_at<?4 AND IIF(open=1, ?1, left_at)>=?3`, now, id, from, to)
		if fail(c, err) {
			return
		}
		type st struct {
			Map                    string  `json:"map"`
			Matches                int     `json:"matches"`
			Secs                   int64   `json:"secs"`
			Players, Left, Started int     `json:"-"`
			Change                 int     `json:"-"`
			AvgPlayers             float64 `json:"avg_players"`
			LeftRate               any     `json:"left_rate"`  // share of the players who left before the end
			AvgChange              any     `json:"avg_change"` // players at the end minus players at the start
		}
		by := map[string]*st{}
		const margin = 60 // joins/leaves around a map change are not "during the match"
		for _, m := range ms {
			s := by[m.Map]
			if s == nil {
				s = &st{Map: m.Map}
				by[m.Map] = s
			}
			s.Matches++
			s.Secs += m.End - m.Start
			present := func(t int64) (n int) {
				for _, x := range cs {
					if x["a"].(int64) <= t && t < x["b"].(int64) {
						n++
					}
				}
				return
			}
			for _, x := range cs {
				a, b := x["a"].(int64), x["b"].(int64)
				if a >= m.End || b <= m.Start {
					continue
				}
				s.Players++
				if x["open"].(int64) == 0 && b > m.Start+margin && b < m.End-margin {
					s.Left++
				}
			}
			if m.End-m.Start > 2*margin {
				s.Started += present(m.Start + margin)
				s.Change += present(m.End-margin) - present(m.Start+margin)
			}
		}
		out := []*st{}
		for _, s := range by {
			if len(cs) > 0 && s.Players > 0 {
				s.AvgPlayers = float64(s.Players) / float64(s.Matches)
				s.LeftRate = float64(s.Left) / float64(s.Players)
				s.AvgChange = float64(s.Change) / float64(s.Matches)
			}
			out = append(out, s)
		}
		sort.Slice(out, func(i, j int) bool {
			return out[i].Matches > out[j].Matches || out[i].Matches == out[j].Matches && out[i].Map < out[j].Map
		})
		c.JSON(200, gin.H{"maps": out, "source": map[bool]string{true: "enterleave", false: "none"}[len(cs) > 0]})
	})

	// Per map, from the status polls: plays, time, players (maprank.go). Feeds the Maps tab.
	g.GET("/maprank", func(c *gin.Context) {
		from, to := span(c, "30d")
		ranks, err := rankMaps(db, c.Param("id"), from, to)
		if !fail(c, err) {
			c.JSON(200, gin.H{"maps": ranks})
		}
	})

	// End-of-match map votes of the period (mod's MV / MVW lines): how often each map was offered, voted and chosen.
	g.GET("/mapvotes", func(c *gin.Context) {
		from, to := span(c, "30d")
		r, err := rows(db, `SELECT map, mode, COUNT(*) AS offered, SUM(votes) AS votes, SUM(chosen) AS chosen FROM map_votes
			WHERE server_id=? AND ts>=? AND ts<? GROUP BY map, mode ORDER BY votes DESC, chosen DESC`, c.Param("id"), from, to)
		if !fail(c, err) {
			c.JSON(200, gin.H{"votes": r})
		}
	})

	// !report from the chat (logtail/commands.go)
	g.GET("/reports", func(c *gin.Context) {
		r, err := rows(db, `SELECT id, ts, reporter_key, reporter_name, target, reason, handled_by, handled_at FROM reports
			WHERE server_id=? ORDER BY handled_at IS NOT NULL, id DESC LIMIT 100`, c.Param("id"))
		if !fail(c, err) {
			c.JSON(200, gin.H{"reports": r})
		}
	})
	g.POST("/reports/:rid/done", sameOrigin, func(c *gin.Context) {
		var target string
		db.QueryRow(`SELECT target FROM reports WHERE id=? AND server_id=?`, c.Param("rid"), c.Param("id")).Scan(&target)
		c.Set("audit", target)
		_, err := db.Exec(`UPDATE reports SET handled_by=?, handled_at=? WHERE id=? AND server_id=?`, c.GetString("user"), time.Now().Unix(), c.Param("rid"), c.Param("id"))
		if !fail(c, err) {
			c.JSON(200, gin.H{"ok": true})
		}
	})

	// Welcome message sent in game (tell) when this player connects; admin only. Empty = none. {name} = the player's name.
	g.PUT("/welcome", sameOrigin, func(c *gin.Context) {
		var req struct{ Key, Message string }
		if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" || len(req.Message) > 150 {
			c.JSON(400, gin.H{"error": "message invalide (150 caractères max)"})
			return
		}
		key := resolveKey(db, c.Param("id"), req.Key)
		who := strings.TrimPrefix(key, "n:") // audit detail: the player's latest name rather than a GUID
		db.QueryRow(`SELECT name_clean FROM player_names WHERE server_id=? AND guid=? ORDER BY last_seen DESC LIMIT 1`, c.Param("id"), key).Scan(&who)
		c.Set("audit", who+" : "+req.Message)
		var err error
		if strings.TrimSpace(req.Message) == "" {
			_, err = db.Exec(`DELETE FROM welcome_messages WHERE server_id=? AND key=?`, c.Param("id"), key)
		} else {
			_, err = db.Exec(`INSERT INTO welcome_messages VALUES(?,?,?,?) ON CONFLICT(server_id, key) DO UPDATE SET message=excluded.message,
				updated_at=excluded.updated_at`, c.Param("id"), key, strings.TrimSpace(req.Message), time.Now().Unix())
		}
		if !fail(c, err) {
			c.JSON(200, gin.H{"ok": true})
		}
	})

	// Possible ban evasion (GUID spoofer): players active in the last 30 days scored against every ban (internal/evasion).
	g.GET("/evasion", func(c *gin.Context) {
		r, err := evasion.Find(db, mustID(c), time.Now().AddDate(0, 0, -30).Unix())
		if !fail(c, err) {
			c.JSON(200, gin.H{"evasions": r, "alert": evasion.Alert})
		}
	})
}

// tagBadge: most kills with a KillEvent tag of the mod (ns, qs, wb) in the period.
func tagBadge(tag string) string {
	return `SELECT killer_key AS key, (SELECT killer_name FROM kills k2 WHERE k2.server_id=?1 AND k2.killer_key=x.killer_key ORDER BY ts DESC LIMIT 1) AS name_raw,
		COUNT(*) AS value FROM kills x WHERE server_id=?1 AND ts>=?2 AND ts<?3 AND teamkill=0 AND ','||tags||',' LIKE '%,` + tag + `,%'
		GROUP BY killer_key ORDER BY value DESC LIMIT 1`
}

// matchResult: winner and final score of a match: the mod's ME line, else the RE line of its last round with one.
func matchResult(db *sql.DB, m *match) (winner any, allies, axis any) {
	in, args := placeholders(m.Rounds)
	if db.QueryRow(`SELECT winner, score_allies, score_axis FROM match_results WHERE round_id IN (`+in+`) ORDER BY id DESC LIMIT 1`, args...).Scan(&winner, &allies, &axis) == nil {
		return
	}
	db.QueryRow(`SELECT winner, score_allies, score_axis FROM rounds WHERE id IN (`+in+`) AND winner IS NOT NULL ORDER BY id DESC LIMIT 1`, args...).Scan(&winner, &allies, &axis)
	return
}

func mustInt(s string) int64 {
	var n int64
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			return -1
		}
		n = n*10 + int64(ch-'0')
	}
	return n
}
