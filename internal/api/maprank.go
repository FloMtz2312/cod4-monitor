package api

import (
	"database/sql"
	"sort"
)

// mapRank sums up, per map, what the status polls saw while it was running (no logs needed):
// how many times it was loaded, for how long, with how many players, and whether they stayed.
type mapRank struct {
	Map         string  `json:"map"`
	Plays       int     `json:"plays"`       // times the map was loaded
	Secs        int64   `json:"secs"`        // time on the server
	PlayerSecs  int64   `json:"player_secs"` // sum of players x time: overall attendance
	EmptySecs   int64   `json:"empty_secs"`  // time with nobody on the server
	MaxPlayers  int     `json:"max_players"` // peak
	AvgPlayers  float64 `json:"avg_players"` // time-weighted average
	AvgChange   any     `json:"avg_change"`  // players at the end minus players at the start, per play (plays > 2 min)
	LastPlayed  int64   `json:"last_played"` // last poll on this map
	Offered     int64   `json:"offered"`     // end-of-match vote (mod's MV lines)
	Votes       int64   `json:"votes"`
	Chosen      int64   `json:"chosen"`
	change      int
	changeCount int
}

const (
	pollGapMax = 60  // a longer gap between two polls (monitor stopped) counts as 60 s
	playBreak  = 600 // no poll for 10 min: the next one starts a new play, even on the same map
	playMin    = 120 // plays shorter than this don't count in avg_change
)

func rankMaps(db *sql.DB, id string, from, to int64) ([]*mapRank, error) {
	rs, err := db.Query(`SELECT ts, online, COALESCE(map, ''), COALESCE(players, 0) FROM snapshots
		WHERE server_id=? AND ts>=? AND ts<? ORDER BY ts, id`, id, from, to)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	by := map[string]*mapRank{}
	get := func(m string) *mapRank {
		r := by[m]
		if r == nil {
			r = &mapRank{Map: m}
			by[m] = r
		}
		return r
	}
	var (
		cur                  *mapRank // map of the play in progress, nil when offline
		playStart, prevTS    int64
		startPlayers, prevPl int
	)
	endPlay := func() {
		if cur != nil && prevTS-playStart >= playMin {
			cur.change += prevPl - startPlayers
			cur.changeCount++
		}
		cur = nil
	}
	for rs.Next() {
		var ts, online int64
		var mp string
		var pl int
		if err := rs.Scan(&ts, &online, &mp, &pl); err != nil {
			return nil, err
		}
		if online == 0 || mp == "" {
			endPlay()
			prevTS = ts
			continue
		}
		if cur != nil && cur.Map == mp && ts-prevTS <= playBreak {
			dt := min(ts-prevTS, pollGapMax)
			cur.Secs += dt
			cur.PlayerSecs += dt * int64(prevPl)
			if prevPl == 0 {
				cur.EmptySecs += dt
			}
		} else {
			endPlay()
			cur = get(mp)
			cur.Plays++
			playStart, startPlayers = ts, pl
		}
		cur.MaxPlayers = max(cur.MaxPlayers, pl)
		cur.LastPlayed = ts
		prevTS, prevPl = ts, pl
	}
	if err := rs.Err(); err != nil {
		return nil, err
	}
	endPlay()

	// maps offered in the vote but never played still show up, so they can be spotted
	votes, err := rows(db, `SELECT map, COUNT(*) AS offered, SUM(votes) AS votes, SUM(chosen) AS chosen FROM map_votes
		WHERE server_id=? AND ts>=? AND ts<? GROUP BY map`, id, from, to)
	if err != nil {
		return nil, err
	}
	for _, v := range votes {
		r := get(v["map"].(string))
		r.Offered, _ = v["offered"].(int64)
		r.Votes, _ = v["votes"].(int64)
		r.Chosen, _ = v["chosen"].(int64)
	}

	out := make([]*mapRank, 0, len(by))
	for _, r := range by {
		if r.Secs > 0 {
			r.AvgPlayers = float64(r.PlayerSecs) / float64(r.Secs)
		}
		if r.changeCount > 0 {
			r.AvgChange = float64(r.change) / float64(r.changeCount)
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].PlayerSecs > out[j].PlayerSecs || out[i].PlayerSecs == out[j].PlayerSecs && out[i].Map < out[j].Map
	})
	return out, nil
}
