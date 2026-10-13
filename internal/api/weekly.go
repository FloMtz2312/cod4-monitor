package api

import (
	"database/sql"
	"time"

	"github.com/gin-gonic/gin"
)

// Week sums up a period for the weekly email and the "player of the week" on the home page.
type Week struct {
	From, To   int64
	Players    int      `json:"players"`     // different players seen
	PlaySecs   int64    `json:"play_secs"`   // time played, all players together
	Peak       int      `json:"peak"`        // most players at once
	PeakAt     int64    `json:"peak_at"`     //
	Matches    int      `json:"matches"`     //
	Kills      int      `json:"kills"`       //
	Top        []Ranked `json:"top"`         // best players by kills (5)
	Longest    *Record  `json:"longest"`     // longest kill (mod's KE lines)
	Noscopes   *Record  `json:"noscopes"`    // most noscopes
	Headshots  *Record  `json:"headshots"`   // most headshots
	Map        string   `json:"map"`         // most attended map
	MapPlayers float64  `json:"map_players"` // its average players
}

type Ranked struct {
	Key       string `json:"key"`
	Name      string `json:"name_raw"`
	Kills     int    `json:"kills"`
	Deaths    int    `json:"deaths"`
	Headshots int    `json:"headshots"`
}

type Record struct {
	Key   string  `json:"key"`
	Name  string  `json:"name_raw"`
	Value float64 `json:"value"`
	Extra string  `json:"extra,omitempty"` // weapon / map of the record
}

// LastWeek is the previous Monday-to-Monday week in loc.
func LastWeek(now time.Time, loc *time.Location) (from, to time.Time) {
	t := now.In(loc)
	day := (int(t.Weekday()) + 6) % 7 // Monday = 0
	to = time.Date(t.Year(), t.Month(), t.Day()-day, 0, 0, 0, 0, loc)
	return to.AddDate(0, 0, -7), to
}

func WeekSummary(db *sql.DB, id string, from, to int64) (*Week, error) {
	w := &Week{From: from, To: to}
	err := db.QueryRow(`SELECT COUNT(DISTINCT name_clean), COALESCE(SUM(MIN(last_seen, ?3) - MAX(started_at, ?2)), 0) FROM player_sessions
		WHERE server_id=?1 AND started_at<?3 AND last_seen>=?2`, id, from, to).Scan(&w.Players, &w.PlaySecs)
	if err != nil {
		return nil, err
	}
	var peak sql.NullInt64
	db.QueryRow(`SELECT players, ts FROM snapshots WHERE server_id=? AND ts>=? AND ts<? AND online=1 ORDER BY players DESC, ts LIMIT 1`, id, from, to).
		Scan(&peak, &w.PeakAt)
	w.Peak = int(peak.Int64)
	ms, err := matchGroups(db, id, from, to)
	if err != nil {
		return nil, err
	}
	w.Matches = len(ms)

	rs, err := db.Query(`WITH k AS (SELECT * FROM kills WHERE server_id=?1 AND ts>=?2 AND ts<?3)
		SELECT killer_key, (SELECT killer_name FROM k k2 WHERE k2.killer_key=k.killer_key ORDER BY ts DESC LIMIT 1),
		  COUNT(*), SUM(cause='MOD_HEAD_SHOT'), (SELECT COUNT(*) FROM k k3 WHERE k3.victim_key=k.killer_key)
		FROM k WHERE killer_key IS NOT NULL AND teamkill=0 GROUP BY killer_key
		ORDER BY COUNT(*) DESC, COUNT(*)*1.0/MAX(1, (SELECT COUNT(*) FROM k k3 WHERE k3.victim_key=k.killer_key)) DESC LIMIT 5`, id, from, to)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var r Ranked
		if err := rs.Scan(&r.Key, &r.Name, &r.Kills, &r.Headshots, &r.Deaths); err != nil {
			rs.Close()
			return nil, err
		}
		w.Top = append(w.Top, r)
	}
	rs.Close()
	db.QueryRow(`SELECT COUNT(*) FROM kills WHERE server_id=? AND ts>=? AND ts<? AND killer_key IS NOT NULL AND teamkill=0`, id, from, to).Scan(&w.Kills)

	record := func(q string) *Record {
		var r Record
		var extra sql.NullString
		if db.QueryRow(q, id, from, to).Scan(&r.Key, &r.Name, &r.Value, &extra) != nil || r.Value <= 0 {
			return nil
		}
		r.Extra = extra.String
		return &r
	}
	w.Longest = record(`SELECT killer_key, killer_name, distance, weapon FROM kills
		WHERE server_id=? AND ts>=? AND ts<? AND teamkill=0 AND killer_key IS NOT NULL AND distance IS NOT NULL ORDER BY distance DESC LIMIT 1`)
	w.Noscopes = record(`SELECT killer_key, MAX(killer_name), COUNT(*), NULL FROM kills
		WHERE server_id=? AND ts>=? AND ts<? AND teamkill=0 AND killer_key IS NOT NULL AND ','||tags||',' LIKE '%,ns,%' GROUP BY killer_key ORDER BY COUNT(*) DESC LIMIT 1`)
	w.Headshots = record(`SELECT killer_key, MAX(killer_name), COUNT(*), NULL FROM kills
		WHERE server_id=? AND ts>=? AND ts<? AND teamkill=0 AND killer_key IS NOT NULL AND cause='MOD_HEAD_SHOT' GROUP BY killer_key ORDER BY COUNT(*) DESC LIMIT 1`)

	maps, err := rankMaps(db, id, from, to)
	if err != nil {
		return nil, err
	}
	for _, m := range maps { // sorted by attendance
		if m.Plays > 0 {
			w.Map, w.MapPlayers = m.Map, m.AvgPlayers
			break
		}
	}
	return w, nil
}

// /week: the last 7 days, for the "player of the week" card on the home page (visitors too).
func weekRoutes(g *gin.RouterGroup, db *sql.DB) {
	g.GET("/week", func(c *gin.Context) {
		to := time.Now().Unix()
		w, err := WeekSummary(db, c.Param("id"), to-7*86400, to)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, w)
	})
}
