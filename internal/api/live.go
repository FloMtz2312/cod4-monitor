package api

import (
	"database/sql"
	"net/url"
	"strconv"
	"time"

	"cod4monitor/internal/store"

	"github.com/gin-gonic/gin"
)

// liveRoutes: what the live screen shows besides the server's own status.
func liveRoutes(g *gin.RouterGroup, db *sql.DB) {
	fail := func(c *gin.Context, err error) bool {
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return err != nil
	}
	// /feed: the running match (map, score), its last events (kills, bomb, clutches, round ends), newest first,
	// and per player seen in it: team, key and kills / deaths. Public, like the in-game kill feed.
	g.GET("/feed", func(c *gin.Context) {
		id := c.Param("id")
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
		limit = min(max(limit, 1), 100)
		now := time.Now().Unix()
		match, err := liveMatch(db, id, now)
		if fail(c, err) {
			return
		}
		// events and teams of the running match; without one, of the last 30 minutes (the mod has no RE lines)
		since := now - 1800
		if match != nil {
			since = match["started_at"].(int64)
		}
		events, err := rows(db, `SELECT * FROM (
			SELECT ts, 'kill' AS type, killer_name AS actor, killer_team AS actor_team, victim_name AS target, victim_team AS target_team,
				weapon, cause, distance, tags, teamkill, NULL AS value, NULL AS winner, NULL AS score_allies, NULL AS score_axis
			FROM kills WHERE server_id=?1 AND ts>=?2
			UNION ALL SELECT ts, type, name_raw, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 0, value, NULL, NULL, NULL
			FROM game_events WHERE server_id=?1 AND ts>=?2 AND type IN ('plant', 'defuse', 'clutch')
			UNION ALL SELECT ended_at, 'round', NULL, NULL, NULL, NULL, NULL, NULL, NULL, NULL, 0, NULL, winner, score_allies, score_axis
			FROM rounds WHERE server_id=?1 AND ended_at>=?2 AND winner IS NOT NULL
		) ORDER BY ts DESC LIMIT ?3`, id, since, limit)
		if fail(c, err) {
			return
		}
		seen, err := rows(db, `SELECT name, key, team, ts FROM (
			SELECT killer_name AS name, killer_key AS key, killer_team AS team, ts FROM kills WHERE server_id=?1 AND ts>=?2 AND killer_name IS NOT NULL
			UNION ALL SELECT victim_name, victim_key, victim_team, ts FROM kills WHERE server_id=?1 AND ts>=?2
		) ORDER BY ts`, id, since)
		if fail(c, err) {
			return
		}
		type player struct {
			Key    string `json:"key"`
			Team   string `json:"team"` // the last one seen: teams switch at half time
			Kills  int    `json:"kills"`
			Deaths int    `json:"deaths"`
		}
		players := map[string]*player{}
		for _, s := range seen {
			name, _ := s["name"].(string)
			p := players[name]
			if p == nil {
				p = &player{}
				players[name] = p
			}
			p.Key, _ = s["key"].(string)
			if t, _ := s["team"].(string); t != "" {
				p.Team = t
			}
		}
		counts, err := rows(db, `SELECT killer_name AS name, COUNT(*) AS n, 'k' AS what FROM kills
				WHERE server_id=?1 AND ts>=?2 AND killer_name IS NOT NULL AND teamkill=0 AND killer_name<>victim_name GROUP BY killer_name
			UNION ALL SELECT victim_name, COUNT(*), 'd' FROM kills WHERE server_id=?1 AND ts>=?2 GROUP BY victim_name`, id, since)
		if fail(c, err) {
			return
		}
		for _, r := range counts {
			name, _ := r["name"].(string)
			if p := players[name]; p != nil {
				if r["what"] == "k" {
					p.Kills = int(r["n"].(int64))
				} else {
					p.Deaths = int(r["n"].(int64))
				}
			}
		}
		c.JSON(200, gin.H{"match": match, "events": events, "players": players})
	})
}

// meRoutes: the code given by !me in game. /me/<code> is the short address the player types on a phone; the
// interface then claims the code to open and pin the profile ("It's me").
func meRoutes(r *gin.Engine, st *store.Store) {
	r.GET("/api/claim/:code", func(c *gin.Context) {
		id, key, name, ok := st.ClaimMe(c.Param("code"), time.Now())
		if !ok {
			c.JSON(404, gin.H{"error": "Code inconnu ou expiré : tape !me en jeu pour en avoir un nouveau."})
			return
		}
		c.JSON(200, gin.H{"server_id": id, "key": key, "name_raw": name})
	})
	r.GET("/me/:code", func(c *gin.Context) {
		c.Redirect(302, "/#/moi/"+url.PathEscape(c.Param("code")))
	})
}
