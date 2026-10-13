package api

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// statsRoutes: combat leaderboard, rounds, player profile, shared IPs, activity heatmap, console errors.
// Players are identified by a key: their GUID, or "n:<clean name>" when the server gave none.
func statsRoutes(g *gin.RouterGroup, db *sql.DB) {
	send := func(c *gin.Context, parts map[string]any) {
		out := gin.H{}
		for k, v := range parts {
			if q, ok := v.(sqlQuery); ok {
				r, err := rows(db, q.sql, q.args...)
				if err != nil {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
				v = r
			}
			out[k] = v
		}
		c.JSON(200, out)
	}

	g.GET("/combat", func(c *gin.Context) {
		from, to := span(c, "30d")
		const k = `WITH k AS (SELECT * FROM kills WHERE server_id=?1 AND ts>=?2 AND ts<?3) `
		id := c.Param("id")
		send(c, map[string]any{
			"players": sq(k+`, e AS (
				SELECT killer_key AS key, killer_name AS name, ts, 1-teamkill AS kill, 0 AS death, cause='MOD_HEAD_SHOT' AS hs, teamkill AS tk, 0 AS sui, weapon,
				  IIF(teamkill=0, ','||COALESCE(tags, '')||',', NULL) AS tg, IIF(teamkill=0, distance, NULL) AS dist
				FROM k WHERE killer_key IS NOT NULL
				UNION ALL SELECT victim_key, victim_name, ts, 0, 1, 0, 0, killer_key IS NULL, NULL, NULL, NULL FROM k),
				`+sniperCTEs+`
				SELECT e.key AS key, (SELECT name FROM e e2 WHERE e2.key=e.key ORDER BY ts DESC LIMIT 1) AS name_raw,
				  SUM(kill) AS kills, SUM(death) AS deaths, SUM(hs) AS headshots, SUM(tk) AS teamkills, SUM(sui) AS suicides,
				  (SELECT weapon FROM e e3 WHERE e3.key=e.key AND weapon IS NOT NULL GROUP BY weapon ORDER BY COUNT(*) DESC LIMIT 1) AS weapon,
				  SUM(tg LIKE '%,ns,%') AS noscopes, SUM(tg LIKE '%,qs,%') AS quickscopes, SUM(tg LIKE '%,wb,%') AS wallbangs,
				  MAX(dist) AS longest, AVG(dist) AS avg_distance,
				  a.shots, a.hits, a.dmg_done, a.dmg_taken, a.rounds,
				  ev.clutches, ev.plants, ev.defuses, fb.n AS first_bloods, w.won AS rounds_won, w.played AS rounds_played
				FROM e LEFT JOIN (SELECT key AS akey, SUM(shots) AS shots, SUM(hits) AS hits, SUM(dmg_done) AS dmg_done, SUM(dmg_taken) AS dmg_taken,
				  COUNT(*) AS rounds FROM accuracy WHERE server_id=?1 AND ts>=?2 AND ts<?3 GROUP BY key) a ON a.akey=e.key
				LEFT JOIN ev ON ev.key=e.key LEFT JOIN fb ON fb.key=e.key LEFT JOIN w ON w.key=e.key
				GROUP BY e.key ORDER BY kills DESC, deaths LIMIT 300`, id, from, to),
			"weapons": sq(k+`SELECT weapon, COUNT(*) AS kills, SUM(cause='MOD_HEAD_SHOT') AS headshots FROM k
				WHERE killer_key IS NOT NULL GROUP BY weapon ORDER BY kills DESC`, id, from, to),
			"hitlocs": sq(k+`SELECT hitloc, COUNT(*) AS kills FROM k WHERE killer_key IS NOT NULL GROUP BY hitloc ORDER BY kills DESC`, id, from, to),
		})
	})

	// S&D logs an InitGame per round. Rounds without kills (empty server) are hidden.
	g.GET("/rounds", func(c *gin.Context) {
		from, to := span(c, "30d")
		const r = `WITH r AS (SELECT r.*, COUNT(k.id) AS kills FROM rounds r JOIN kills k ON k.round_id=r.id
			WHERE r.server_id=?1 AND r.started_at>=?2 AND r.started_at<?3 GROUP BY r.id) `
		id := c.Param("id")
		send(c, map[string]any{
			"recent": sq(r+`SELECT id, started_at, ended_at, map, gametype, kills,
				(SELECT killer_name FROM kills k WHERE k.round_id=r.id AND killer_key IS NOT NULL GROUP BY killer_key ORDER BY COUNT(*) DESC LIMIT 1) AS top_killer,
				(SELECT COUNT(*) FROM kills k WHERE k.round_id=r.id AND killer_key IS NOT NULL GROUP BY killer_key ORDER BY COUNT(*) DESC LIMIT 1) AS top_kills
				FROM r ORDER BY started_at DESC LIMIT 100`, id, from, to),
			"maps": sq(r+`SELECT map, COUNT(*) AS rounds, AVG(ended_at-started_at) AS avg_duration_s, AVG(kills) AS avg_kills
				FROM r WHERE ended_at IS NOT NULL GROUP BY map ORDER BY rounds DESC`, id, from, to),
		})
	})

	g.GET("/player", func(c *gin.Context) {
		id, key := c.Param("id"), c.Query("key")
		if key == "" {
			c.JSON(400, gin.H{"error": "key manquante"})
			return
		}
		if name, ok := strings.CutPrefix(key, "n:"); ok { // a name with a known GUID: use the GUID (all names merged)
			var guid string
			if db.QueryRow(`SELECT guid FROM player_names WHERE server_id=? AND name_clean=? ORDER BY last_seen DESC LIMIT 1`, id, name).Scan(&guid) == nil {
				key = guid
			}
		}
		// connections/chat of this player: by GUID, or by name for players without one
		guid, name, byName := key, "", strings.HasPrefix(key, "n:")
		cond := `server_id=?1 AND guid=?2`
		if byName {
			guid, name = "", strings.TrimPrefix(key, "n:")
			cond = `server_id=?1 AND guid IS NULL AND name_clean=?2`
		}
		who := key
		if byName {
			who = name
		}
		now := time.Now().Unix()
		conns, err := rows(db, `SELECT joined_at AS a, COALESCE(left_at, IIF(open=1, ?3, joined_at)) AS b FROM connections WHERE `+cond, id, who, now)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		// seconds played per local hour of day
		hours := make([]int64, 24)
		for _, r := range conns {
			for t, b := r["a"].(int64), r["b"].(int64); t < b; {
				lt := time.Unix(t, 0)
				next := min(lt.Truncate(time.Hour).Add(time.Hour).Unix(), b)
				hours[lt.Hour()] += next - t
				t = next
			}
		}
		names := sq(`SELECT name_raw, name_clean, first_seen, last_seen FROM player_names WHERE server_id=?1 AND guid=?2 ORDER BY last_seen DESC`, id, guid)
		if byName {
			names = sq(`SELECT name_raw, name_clean, MIN(joined_at) AS first_seen, MAX(joined_at) AS last_seen FROM connections WHERE `+cond+` GROUP BY name_clean`, id, name)
		}
		chatCond := `server_id=?1 AND guid=?2`
		if byName {
			chatCond = `server_id=?1 AND name_clean=?2`
		}
		parts := map[string]any{
			"key":   key,
			"hours": hours,
			"names": names,
			"summary": sq(`SELECT COUNT(*) AS sessions, MIN(joined_at) AS first_seen, MAX(COALESCE(left_at, joined_at)) AS last_seen, MAX(open) AS online,
				SUM(MAX(0, COALESCE(left_at, IIF(open=1, ?3, joined_at)) - joined_at)) AS playtime_s FROM connections WHERE `+cond, id, who, now),
			"ips": sq(`SELECT ip, COUNT(*) AS sessions, MIN(joined_at) AS first_seen, MAX(joined_at) AS last_seen FROM connections WHERE `+cond+`
				GROUP BY ip ORDER BY last_seen DESC`, id, who),
			"sessions": sq(`SELECT joined_at, left_at, open, name_raw, ip FROM connections WHERE `+cond+` ORDER BY joined_at DESC LIMIT 100`, id, who),
			"chat":     sq(`SELECT ts, name_raw, team, message FROM chat_messages WHERE `+chatCond+` ORDER BY ts DESC, id DESC LIMIT 100`, id, who),
			"accuracy": sq(`SELECT SUM(shots) AS shots, SUM(hits) AS hits, SUM(dmg_done) AS dmg_done, SUM(dmg_taken) AS dmg_taken,
				COUNT(*) AS rounds FROM accuracy WHERE server_id=?1 AND key=?2`, id, key),
			"note": sq(`SELECT tag, note, updated_at FROM player_notes WHERE server_id=?1 AND key=?2`, id, key),
			"combat": sq(`SELECT
				(SELECT COUNT(*) FROM kills WHERE server_id=?1 AND killer_key=?2 AND teamkill=0) AS kills,
				(SELECT COUNT(*) FROM kills WHERE server_id=?1 AND victim_key=?2) AS deaths,
				(SELECT COUNT(*) FROM kills WHERE server_id=?1 AND killer_key=?2 AND cause='MOD_HEAD_SHOT') AS headshots,
				(SELECT COUNT(*) FROM kills WHERE server_id=?1 AND killer_key=?2 AND teamkill=1) AS teamkills,
				(SELECT COUNT(*) FROM kills WHERE server_id=?1 AND victim_key=?2 AND killer_key IS NULL) AS suicides,
				(SELECT MAX(damage) FROM kills WHERE server_id=?1 AND killer_key=?2 AND damage<1000) AS max_damage`, id, key),
			"weapons": sq(`SELECT weapon, COUNT(*) AS kills, SUM(cause='MOD_HEAD_SHOT') AS headshots FROM kills
				WHERE server_id=?1 AND killer_key=?2 GROUP BY weapon ORDER BY kills DESC`, id, key),
			"hitlocs": sq(`SELECT hitloc, COUNT(*) AS kills FROM kills WHERE server_id=?1 AND killer_key=?2 GROUP BY hitloc ORDER BY kills DESC`, id, key),
			"nemesis": sq(`SELECT killer_key AS key, (SELECT killer_name FROM kills k2 WHERE k2.server_id=?1 AND k2.killer_key=k.killer_key ORDER BY ts DESC LIMIT 1) AS name_raw,
				COUNT(*) AS kills FROM kills k WHERE server_id=?1 AND victim_key=?2 AND killer_key IS NOT NULL AND killer_key<>?2
				GROUP BY killer_key ORDER BY kills DESC LIMIT 5`, id, key),
			"victims": sq(`SELECT victim_key AS key, (SELECT victim_name FROM kills k2 WHERE k2.server_id=?1 AND k2.victim_key=k.victim_key ORDER BY ts DESC LIMIT 1) AS name_raw,
				COUNT(*) AS kills FROM kills k WHERE server_id=?1 AND killer_key=?2 AND victim_key<>?2
				GROUP BY victim_key ORDER BY kills DESC LIMIT 5`, id, key),
			// other GUIDs that connected from one of this player's IPs
			"linked": sq(`SELECT c2.guid AS key, c2.ip, (SELECT name_raw FROM connections c3 WHERE c3.server_id=?1 AND c3.guid=c2.guid ORDER BY joined_at DESC LIMIT 1) AS name_raw,
				COUNT(*) AS sessions FROM connections c2 WHERE c2.server_id=?1 AND c2.guid IS NOT NULL AND c2.guid<>?2
				AND c2.ip IN (SELECT ip FROM connections WHERE server_id=?1 AND guid=?2) GROUP BY c2.guid, c2.ip`, id, guid),
			"bans": sq(`SELECT ts, duration, reason FROM bans WHERE server_id=?1 AND guid=?2 ORDER BY ts DESC`, id, guid),
			"best_score": sq(`SELECT MAX(best_score) AS best FROM player_sessions WHERE server_id=?1 AND name_clean IN
				(SELECT name_clean FROM player_names WHERE server_id=?1 AND guid=?2 UNION SELECT ?3)`, id, guid, name),
		}
		// sniper stats of the mod (KE / CL / P_P / P_D / RE / MS lines), all time
		parts["sniper"] = sq(`WITH k AS (SELECT * FROM kills WHERE server_id=?1 AND (killer_key=?2 OR victim_key=?2)),
			ev AS (SELECT key, SUM(type='clutch') AS clutches, SUM(type='plant') AS plants, SUM(type='defuse') AS defuses
				FROM game_events WHERE server_id=?1 AND key=?2 GROUP BY key)
			SELECT SUM(killer_key=?2 AND teamkill=0 AND ','||tags||',' LIKE '%,ns,%') AS noscopes, SUM(killer_key=?2 AND teamkill=0 AND ','||tags||',' LIKE '%,qs,%') AS quickscopes,
			  SUM(killer_key=?2 AND teamkill=0 AND ','||tags||',' LIKE '%,wb,%') AS wallbangs, MAX(IIF(killer_key=?2 AND teamkill=0, distance, NULL)) AS longest,
			  AVG(IIF(killer_key=?2 AND teamkill=0, distance, NULL)) AS avg_distance, SUM(IIF(killer_key=?2 AND teamkill=0, points, 0)) AS points,
			  (SELECT clutches FROM ev WHERE key=?2) AS clutches, (SELECT plants FROM ev WHERE key=?2) AS plants, (SELECT defuses FROM ev WHERE key=?2) AS defuses,
			  (SELECT SUM(r.winner=p.team) FROM (SELECT DISTINCT round_id, IIF(killer_key=?2, killer_team, victim_team) AS team FROM k) p JOIN rounds r ON r.id=p.round_id
			    AND r.winner IN ('allies', 'axis') WHERE p.team IN ('allies', 'axis')) AS rounds_won,
			  (SELECT COUNT(*) FROM (SELECT DISTINCT round_id, IIF(killer_key=?2, killer_team, victim_team) AS team FROM k) p JOIN rounds r ON r.id=p.round_id
			    AND r.winner IN ('allies', 'axis') WHERE p.team IN ('allies', 'axis')) AS rounds_played,
			  (SELECT level FROM match_scores WHERE server_id=?1 AND key=?2 ORDER BY ts DESC LIMIT 1) AS level,
			  (SELECT prestige FROM match_scores WHERE server_id=?1 AND key=?2 ORDER BY ts DESC LIMIT 1) AS prestige
			FROM k`, id, key)
		// the name to show: the latest one seen (names of the log, else of the kills)
		var display sql.NullString
		db.QueryRow(`SELECT name_raw FROM (SELECT name_raw, last_seen AS ts FROM player_names WHERE server_id=?1 AND guid=?2
			UNION ALL SELECT killer_name, MAX(ts) FROM kills WHERE server_id=?1 AND killer_key=?2
			UNION ALL SELECT victim_name, MAX(ts) FROM kills WHERE server_id=?1 AND victim_key=?2)
			WHERE name_raw IS NOT NULL ORDER BY ts DESC LIMIT 1`, id, key).Scan(&display)
		parts["name_raw"] = display.String
		if !display.Valid {
			parts["name_raw"] = name
		}
		if role(c) == roleAdmin { // the admin's welcome message for this player
			var msg string
			db.QueryRow(`SELECT message FROM welcome_messages WHERE server_id=? AND key=?`, id, key).Scan(&msg)
			parts["welcome"] = msg
		}
		if role(c) == roleVisitor { // private: IPs, sessions, chat, admin notes, bans
			for _, k := range []string{"ips", "sessions", "chat", "linked", "bans"} {
				delete(parts, k)
			}
			if !grants(db, c.GetString("user"), mustID(c))["notes"] { // the notes permission sees (and edits) the note only
				delete(parts, "note")
			}
		}
		send(c, parts)
	})

	// IPs used by several GUIDs: alternate accounts, or ban evasion.
	g.GET("/alts", func(c *gin.Context) {
		send(c, map[string]any{"alts": sq(`SELECT ip, guid AS key, COUNT(*) AS sessions, MAX(joined_at) AS last_seen,
			(SELECT name_raw FROM connections c3 WHERE c3.server_id=c.server_id AND c3.guid=c.guid ORDER BY joined_at DESC LIMIT 1) AS name_raw
			FROM connections c WHERE server_id=?1 AND guid IS NOT NULL AND ip IN
			  (SELECT ip FROM connections WHERE server_id=?1 AND guid IS NOT NULL GROUP BY ip HAVING COUNT(DISTINCT guid)>1)
			GROUP BY ip, guid ORDER BY ip, last_seen DESC`, c.Param("id"))})
	})

	// Average players per weekday (0 = Monday) and local hour, from connections or else from polls.
	g.GET("/heatmap", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		from = time.Unix(from, 0).Truncate(time.Hour).Unix()
		now := time.Now().Unix()
		connRows, err := rows(db, `SELECT joined_at AS a, IIF(open=1, ?1, left_at) AS b FROM connections
			WHERE server_id=?2 AND (open=1 OR left_at IS NOT NULL) AND joined_at<?4 AND IIF(open=1, ?1, left_at)>=?3`, now, id, from, to)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		perHour := map[int64]float64{}
		if len(connRows) > 0 {
			conns := make([][2]int64, len(connRows))
			for i, r := range connRows {
				conns[i] = [2]int64{r["a"].(int64), r["b"].(int64)}
			}
			for t := from; t < to; t += 3600 {
				perHour[t], _ = occupancy(conns, t, min(t+3600, to))
			}
		} else {
			snaps, err := rows(db, `SELECT ts/3600*3600 AS h, AVG(players) AS p FROM snapshots
				WHERE server_id=? AND ts>=? AND ts<? AND online=1 GROUP BY h`, id, from, to)
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			for _, r := range snaps {
				perHour[r["h"].(int64)] = r["p"].(float64)
			}
		}
		var sum [7][24]float64
		var n [7][24]int
		for t, v := range perHour {
			lt := time.Unix(t, 0)
			wd := (int(lt.Weekday()) + 6) % 7
			sum[wd][lt.Hour()] += v
			n[wd][lt.Hour()]++
		}
		cells := make([][]any, 7)
		for d := range cells {
			cells[d] = make([]any, 24)
			for h := range cells[d] {
				if n[d][h] > 0 {
					cells[d][h] = sum[d][h] / float64(n[d][h])
				}
			}
		}
		c.JSON(200, gin.H{"cells": cells, "source": map[bool]string{true: "enterleave", false: "udp"}[len(connRows) > 0]})
	})

	g.GET("/errors", func(c *gin.Context) {
		send(c, map[string]any{"errors": sq(`SELECT message, count, first_seen, last_seen FROM console_errors
			WHERE server_id=? ORDER BY last_seen DESC LIMIT 100`, c.Param("id"))})
	})
}

// sniperCTEs (after "WITH k AS (kills of the period) ,"): per player key, over the period ?1 server, [?2, ?3):
//
//	ev: clutches / plants / defuses (game_events), fb: first bloods (first enemy kill of a round),
//	w: rounds won / played (team of the player in each round from its kill lines, winner from the RE line of the mod)
const sniperCTEs = `ev AS (SELECT key, SUM(type='clutch') AS clutches, SUM(type='plant') AS plants, SUM(type='defuse') AS defuses
		FROM game_events WHERE server_id=?1 AND ts>=?2 AND ts<?3 GROUP BY key),
	fb AS (SELECT key, COUNT(*) AS n FROM (SELECT (SELECT killer_key FROM kills k3 WHERE k3.round_id=r.id AND k3.killer_key IS NOT NULL AND k3.teamkill=0
		ORDER BY k3.id LIMIT 1) AS key FROM rounds r WHERE r.server_id=?1 AND r.started_at>=?2 AND r.started_at<?3) WHERE key IS NOT NULL GROUP BY key),
	w AS (SELECT p.key, SUM(r.winner=p.team) AS won, COUNT(*) AS played FROM (
		SELECT DISTINCT round_id, killer_key AS key, killer_team AS team FROM k WHERE killer_key IS NOT NULL AND killer_team IN ('allies', 'axis')
		UNION SELECT round_id, victim_key, victim_team FROM k WHERE victim_team IN ('allies', 'axis')) p
		JOIN rounds r ON r.id=p.round_id AND r.winner IN ('allies', 'axis') GROUP BY p.key)`

type sqlQuery struct {
	sql  string
	args []any
}

func sq(sql string, args ...any) sqlQuery { return sqlQuery{sql, args} }
