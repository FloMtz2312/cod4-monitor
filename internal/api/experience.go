package api

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
)

type formStats struct {
	Kills     int64    `json:"kills"`
	Deaths    int64    `json:"deaths"`
	Headshots int64    `json:"headshots"`
	Shots     int64    `json:"shots"`
	Hits      int64    `json:"hits"`
	KD        float64  `json:"kd"`
	Accuracy  *float64 `json:"accuracy"`
}

func playerForm(db *sql.DB, id, key string, from, to int64) (formStats, error) {
	var f formStats
	// Whole UTC days use materialized aggregates, boundaries retain exact timestamps.
	a, b := (from+86399)/86400, to/86400
	if b < a {
		b = a
	}
	err := db.QueryRow(`SELECT COALESCE(SUM(kills),0),COALESCE(SUM(deaths),0),COALESCE(SUM(headshots),0)
 FROM daily_combat WHERE server_id=? AND key=? AND day>=? AND day<?`, id, key, a, b).Scan(&f.Kills, &f.Deaths, &f.Headshots)
	if err != nil {
		return f, err
	}
	var k, d, h int64
	err = db.QueryRow(`SELECT COALESCE(SUM(killer_key=?2 AND teamkill=0),0),COALESCE(SUM(victim_key=?2),0),
 COALESCE(SUM(killer_key=?2 AND teamkill=0 AND cause='MOD_HEAD_SHOT'),0)
 FROM kills WHERE server_id=?1 AND ts>=?3 AND ts<?4 AND (ts<?5 OR ts>=?6) AND (killer_key=?2 OR victim_key=?2)`, id, key, from, to, a*86400, b*86400).Scan(&k, &d, &h)
	if err != nil {
		return f, err
	}
	f.Kills += k
	f.Deaths += d
	f.Headshots += h
	err = db.QueryRow(`SELECT COALESCE(SUM(shots),0),COALESCE(SUM(hits),0) FROM accuracy WHERE server_id=? AND key=? AND ts>=? AND ts<?`, id, key, from, to).Scan(&f.Shots, &f.Hits)
	f.KD = float64(f.Kills) / float64(max(1, f.Deaths))
	if f.Shots > 0 {
		v := 100 * float64(f.Hits) / float64(f.Shots)
		f.Accuracy = &v
	}
	return f, err
}
func experienceRoutes(g *gin.RouterGroup, db *sql.DB) {
	g.GET("/form", func(c *gin.Context) {
		id, key := c.Param("id"), resolveKey(db, c.Param("id"), c.Query("key"))
		if key == "" || len(key) > 200 {
			c.JSON(400, gin.H{"error": "joueur manquant"})
			return
		}
		to := time.Now().Unix()
		from := to - 7*86400
		current, err := playerForm(db, id, key, from, to)
		if err != nil {
			c.JSON(500, gin.H{"error": "statistiques indisponibles"})
			return
		}
		previous, err := playerForm(db, id, key, from-7*86400, from)
		if err != nil {
			c.JSON(500, gin.H{"error": "statistiques indisponibles"})
			return
		}
		recent, err := rows(db, `SELECT r.id,r.started_at,r.map,
   SUM(k.killer_key=?2 AND k.teamkill=0) AS kills,SUM(k.victim_key=?2) AS deaths,
   SUM(k.killer_key=?2 AND k.teamkill=0 AND k.cause='MOD_HEAD_SHOT') AS headshots
   FROM rounds r JOIN kills k ON k.round_id=r.id WHERE r.server_id=?1 AND (k.killer_key=?2 OR k.victim_key=?2)
   GROUP BY r.id ORDER BY r.started_at DESC,r.id DESC LIMIT 10`, id, key)
		if err != nil {
			c.JSON(500, gin.H{"error": "historique indisponible"})
			return
		}
		out := gin.H{"current": current, "previous": previous, "recent": recent, "from": from, "to": to, "sample_sufficient": current.Kills+current.Deaths >= 20 && previous.Kills+previous.Deaths >= 20}
		if other := c.Query("compare"); other != "" && len(other) <= 200 {
			f, e := playerForm(db, id, resolveKey(db, id, other), from, to)
			if e != nil {
				c.JSON(500, gin.H{"error": "comparaison indisponible"})
				return
			}
			out["comparison"] = f
		}
		c.JSON(200, out)
	})
	g.GET("/challenges", func(c *gin.Context) {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM servers WHERE id=?)`, c.Param("id")).Scan(&exists); err != nil {
			c.Status(500)
			return
		}
		if !exists {
			c.Status(404)
			return
		}
		now := time.Now().UTC()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, -(int(now.Weekday())+6)%7).Unix()
		end := start + 7*86400
		id := c.Param("id")
		// Freeze targets per week: changing defaults cannot rewrite past achievements.
		for _, x := range []struct {
			code   string
			target int
		}{{"kills", 1000}, {"headshots", 200}, {"objectives", 50}} {
			_, err := db.Exec(`INSERT OR IGNORE INTO community_weeks(server_id,week,metric,target)
    VALUES(?,?,?,COALESCE((SELECT target FROM community_goals WHERE server_id=? AND metric=?),?))`, id, start, x.code, id, x.code, x.target)
			if err != nil {
				c.JSON(500, gin.H{"error": "défis indisponibles"})
				return
			}
		}
		progress, err := rows(db, `SELECT metric,target FROM community_weeks WHERE server_id=? AND week=? ORDER BY metric`, id, start)
		if err != nil {
			c.JSON(500, gin.H{"error": "défis indisponibles"})
			return
		}
		var kills, hs, objectives, players int64
		if err = db.QueryRow(`SELECT COALESCE(SUM(kills),0),COALESCE(SUM(headshots),0),COUNT(DISTINCT CASE WHEN kills>0 THEN key END) FROM daily_combat WHERE server_id=? AND day>=? AND day<?`, id, start/86400, end/86400).Scan(&kills, &hs, &players); err != nil {
			c.Status(500)
			return
		}
		if err = db.QueryRow(`SELECT COUNT(*) FROM game_events WHERE server_id=? AND ts>=? AND ts<? AND type IN ('plant','defuse','hq_capture','hq_destroy','flag')`, id, start, end).Scan(&objectives); err != nil {
			c.Status(500)
			return
		}
		values := map[string]int64{"kills": kills, "headshots": hs, "objectives": objectives}
		for _, p := range progress {
			p["progress"] = values[p["metric"].(string)]
			p["completed"] = p["progress"].(int64) >= p["target"].(int64)
		}
		records, err := rows(db, `SELECT day*86400 AS ts,SUM(kills) AS kills,SUM(headshots) AS headshots FROM daily_combat WHERE server_id=? GROUP BY day ORDER BY kills DESC,day DESC LIMIT 5`, id)
		if err != nil {
			c.Status(500)
			return
		}
		leaders, err := rows(db, `SELECT key,SUM(kills) AS kills,SUM(headshots) AS headshots,
   COALESCE((SELECT killer_name FROM kills k WHERE k.server_id=?1 AND k.killer_key=d.key ORDER BY ts DESC,id DESC LIMIT 1),key) AS name_raw
   FROM daily_combat d WHERE server_id=?1 AND day>=?2 AND day<?3 GROUP BY key HAVING SUM(kills)>0 ORDER BY kills DESC,key LIMIT 5`, id, start/86400, end/86400)
		if err != nil {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"from": start, "to": end, "timezone": "UTC", "challenges": progress, "contributors": players, "leaders": leaders, "daily_records": records})
	})
	g.PUT("/challenges", sameOrigin, func(c *gin.Context) {
		var exists bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM servers WHERE id=?)`, c.Param("id")).Scan(&exists); err != nil {
			c.Status(500)
			return
		}
		if !exists {
			c.Status(404)
			return
		}
		var req struct {
			Kills      int `json:"kills"`
			Headshots  int `json:"headshots"`
			Objectives int `json:"objectives"`
		}
		if c.ShouldBindJSON(&req) != nil || req.Kills < 1 || req.Headshots < 1 || req.Objectives < 1 || req.Kills > 10000000 || req.Headshots > 10000000 || req.Objectives > 1000000 {
			c.JSON(400, gin.H{"error": "objectifs invalides"})
			return
		}
		tx, err := db.Begin()
		if err != nil {
			c.Status(500)
			return
		}
		defer tx.Rollback()
		for metric, target := range map[string]int{"kills": req.Kills, "headshots": req.Headshots, "objectives": req.Objectives} {
			if _, err = tx.Exec(`INSERT INTO community_goals(server_id,metric,target) VALUES(?,?,?) ON CONFLICT(server_id,metric) DO UPDATE SET target=excluded.target`, c.Param("id"), metric, target); err != nil {
				c.Status(500)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			c.Status(500)
			return
		}
		c.Set("audit", "objectifs de la prochaine semaine")
		c.JSON(200, gin.H{"ok": true, "effective": "next_week"})
	})
}

// One ordered event stream per round, with stable source IDs for simultaneous events.
func matchTimeline(db *sql.DB, id string, roundIDs []int64) ([]map[string]any, error) {
	in, args := placeholders(roundIDs)
	args = append([]any{id}, args...)
	return rows(db, `WITH events AS (
 SELECT round_id,ts,id,'kill' AS type,killer_name AS actor,victim_name AS target,weapon,cause,teamkill,CAST(NULL AS INTEGER) AS value FROM kills WHERE server_id=? AND round_id IN (`+in+`)
 UNION ALL SELECT round_id,ts,id,type,name_raw,NULL,NULL,NULL,0,value FROM game_events WHERE round_id IN (`+in+`))
 SELECT * FROM events ORDER BY round_id,ts,type,id LIMIT 5001`, append(args, args[1:]...)...)
}
