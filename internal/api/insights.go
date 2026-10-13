package api

import (
	"database/sql"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

// insightRoutes: player pings, player retention, round length and attack / defense wins per map.
// Everything comes from data the monitor already stores (status polls, connections, the mod's RE lines).
func insightRoutes(g *gin.RouterGroup, db *sql.DB) {
	fail := func(c *gin.Context, err error) bool {
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
		}
		return err != nil
	}

	// Ping of each player as the status polls saw it (999 = connecting or lagging out, ignored).
	g.GET("/pings", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "7d")
		players, err := rows(db, `SELECT ps.name_clean, MAX(ps.name_raw) AS name_raw,
			(SELECT guid FROM player_names pn WHERE pn.server_id=?1 AND pn.name_clean=ps.name_clean ORDER BY last_seen DESC LIMIT 1) AS guid,
			COUNT(*) AS samples, AVG(ps.ping) AS avg, MAX(ps.ping) AS max, AVG(ps.ping>=?4) AS high
			FROM player_sightings ps JOIN snapshots s ON s.id=ps.snapshot_id
			WHERE s.server_id=?1 AND s.ts>=?2 AND s.ts<?3 AND ps.ping>0 AND ps.ping<999
			GROUP BY ps.name_clean HAVING COUNT(*)>=4 ORDER BY avg DESC LIMIT 200`, id, from, to, highPing)
		if fail(c, err) {
			return
		}
		buckets, err := rows(db, `SELECT s.ts/3600*3600 AS h, SUM(ps.ping) AS sum, COUNT(*) AS n
			FROM player_sightings ps JOIN snapshots s ON s.id=ps.snapshot_id
			WHERE s.server_id=? AND s.ts>=? AND s.ts<? AND ps.ping>0 AND ps.ping<999 GROUP BY h`, id, from, to)
		if fail(c, err) {
			return
		}
		var hourSum [24]float64
		var hourN [24]int64
		daySum, dayN := map[string]float64{}, map[string]int64{}
		var all float64
		var allN int64
		for _, b := range buckets {
			lt := time.Unix(b["h"].(int64), 0)
			s, n := num(b["sum"]), b["n"].(int64)
			hourSum[lt.Hour()] += s
			hourN[lt.Hour()] += n
			d := lt.Format("2006-01-02")
			daySum[d] += s
			dayN[d] += n
			all += s
			allN += n
		}
		hours := make([]any, 24)
		for h := range hours {
			if hourN[h] > 0 {
				hours[h] = hourSum[h] / float64(hourN[h])
			}
		}
		days := []gin.H{}
		for d, n := range dayN {
			days = append(days, gin.H{"day": d, "avg": daySum[d] / float64(n), "samples": n})
		}
		sort.Slice(days, func(i, j int) bool { return days[i]["day"].(string) < days[j]["day"].(string) })
		var avg any
		if allN > 0 {
			avg = all / float64(allN)
		}
		c.JSON(200, gin.H{"players": players, "hours": hours, "days": days, "avg": avg, "high": highPing})
	})

	g.GET("/retention", func(c *gin.Context) {
		from, to := span(c, "30d")
		r, err := retention(db, c.Param("id"), from, to, time.Now())
		if !fail(c, err) {
			c.JSON(200, r)
		}
	})

	// The match running now: its map and the score of its last finished round (the mod's RE lines).
	g.GET("/live", func(c *gin.Context) {
		m, err := liveMatch(db, c.Param("id"), time.Now().Unix())
		if !fail(c, err) {
			c.JSON(200, gin.H{"match": m})
		}
	})

	// Per map: S&D rounds (with a result from the mod's RE line), their length, attack / defense wins, and match length.
	g.GET("/maprounds", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		maps, err := rows(db, `SELECT map, COUNT(*) AS rounds, AVG(ended_at-started_at) AS avg_s,
			MIN(ended_at-started_at) AS min_s, MAX(ended_at-started_at) AS max_s,
			SUM(COALESCE(attackers, '')<>'' AND winner=attackers) AS attack,
			SUM(COALESCE(attackers, '')<>'' AND winner IN ('allies', 'axis') AND winner<>attackers) AS defense
			FROM rounds WHERE server_id=? AND started_at>=? AND started_at<? AND ended_at IS NOT NULL AND winner IS NOT NULL
			GROUP BY map ORDER BY rounds DESC`, id, from, to)
		if fail(c, err) {
			return
		}
		ms, err := matchGroups(db, id, from, to)
		if fail(c, err) {
			return
		}
		type agg struct{ n, secs, rounds int64 }
		by := map[string]*agg{}
		for _, m := range ms {
			a := by[m.Map]
			if a == nil {
				a = &agg{}
				by[m.Map] = a
			}
			a.n++
			a.secs += m.End - m.Start
			a.rounds += int64(m.RoundCount)
		}
		for _, m := range maps {
			if a := by[m["map"].(string)]; a != nil {
				m["matches"] = a.n
				m["match_avg_s"] = float64(a.secs) / float64(a.n)
				m["match_rounds"] = float64(a.rounds) / float64(a.n)
			}
		}
		c.JSON(200, gin.H{"maps": maps})
	})
}

const highPing = 150 // ms: above this a sniper duel is already lost

func num(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}

// Retention thresholds.
const (
	visitMin     = 60 // a visit shorter than this (map load failed, wrong server) doesn't count
	returnWithin = 7  // days: a new player "came back" if they played another day within this
	regularDays  = 4  // days played to count as a regular
	goneAfter    = 10 // days without playing: a regular who stopped coming
	goneMax      = 90 // older than this: forgotten
	retWeeks     = 12 // weeks of the trend
	dayFmt       = "2006-01-02"
	weekSecs     = 7 * 86400
	returnSecs   = returnWithin * 86400
	goneSecs     = goneAfter * 86400
	goneMaxSecs  = goneMax * 86400
	visitMinSecs = visitMin
)

type visitor struct {
	key, name         string
	first, last, play int64
	days              map[string]bool
	visits            [][2]int64 // start and end of each visit, in order
}

// visitors reads every visit ever made (connections from enterleave.log, else the status poll sessions).
func visitors(db *sql.DB, id string, now int64) (map[string]*visitor, string, error) {
	q := `SELECT COALESCE(guid, 'n:'||name_clean) AS k, name_raw, joined_at AS a, COALESCE(left_at, IIF(open=1, ?2, joined_at)) AS b
		FROM connections WHERE server_id=?1 ORDER BY joined_at`
	src := "enterleave"
	var has bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM connections WHERE server_id=?)`, id).Scan(&has); err != nil {
		return nil, "", err
	}
	if !has {
		src = "udp"
		q = `WITH ng AS (SELECT name_clean, guid, MAX(last_seen) FROM player_names WHERE server_id=?1 GROUP BY name_clean)
			SELECT COALESCE(ng.guid, 'n:'||p.name_clean) AS k, p.name_raw, p.started_at AS a, p.last_seen AS b
			FROM player_sessions p LEFT JOIN ng USING(name_clean) WHERE p.server_id=?1 ORDER BY p.started_at`
	}
	args := []any{id}
	if has {
		args = append(args, now)
	}
	rs, err := db.Query(q, args...)
	if err != nil {
		return nil, "", err
	}
	defer rs.Close()
	out := map[string]*visitor{}
	for rs.Next() {
		var k, name string
		var a, b int64
		if err := rs.Scan(&k, &name, &a, &b); err != nil {
			return nil, "", err
		}
		// unknown end (crash before monitoring): count the visit, not its length
		if b > a && b-a < visitMinSecs {
			continue
		}
		v := out[k]
		if v == nil {
			v = &visitor{key: k, first: a, days: map[string]bool{}}
			out[k] = v
		}
		v.name, v.last = name, max(v.last, b)
		v.play += max(0, b-a)
		v.days[time.Unix(a, 0).Format(dayFmt)] = true
		v.visits = append(v.visits, [2]int64{a, b})
	}
	return out, src, rs.Err()
}

// returned: played again on another day within returnWithin days of the first visit.
func (v *visitor) returned() bool {
	firstDay := time.Unix(v.first, 0).Format(dayFmt)
	for _, x := range v.visits {
		if x[0]-v.first > returnSecs {
			break
		}
		if time.Unix(x[0], 0).Format(dayFmt) != firstDay {
			return true
		}
	}
	return false
}

func retention(db *sql.DB, id string, from, to int64, now time.Time) (gin.H, error) {
	vs, src, err := visitors(db, id, now.Unix())
	if err != nil {
		return nil, err
	}
	n := now.Unix()
	active := func(v *visitor, a, b int64) bool {
		for _, x := range v.visits {
			if x[0] < b && x[1] >= a {
				return true
			}
		}
		return false
	}

	// period summary
	var newN, activeN, back, measurable, returned int
	newList := []gin.H{}
	for _, v := range vs {
		if !active(v, from, to) {
			continue
		}
		activeN++
		if v.first < from {
			back++
			continue
		}
		newN++
		ret := v.returned()
		item := gin.H{"key": v.key, "name_raw": v.name, "first_seen": v.first, "last_seen": v.last, "days": len(v.days), "playtime_s": v.play}
		if n-v.first >= returnSecs {
			measurable++
			if ret {
				returned++
			}
			item["returned"] = ret
		} // else: too early to tell (null)
		newList = append(newList, item)
	}
	sort.Slice(newList, func(i, j int) bool { return newList[i]["first_seen"].(int64) > newList[j]["first_seen"].(int64) })
	if len(newList) > 50 {
		newList = newList[:50]
	}

	// weekly trend (weeks start on Monday, local time)
	lt := now
	monday := time.Date(lt.Year(), lt.Month(), lt.Day(), 0, 0, 0, 0, lt.Location()).AddDate(0, 0, -((int(lt.Weekday()) + 6) % 7))
	weeks := []gin.H{}
	for i := retWeeks - 1; i >= 0; i-- {
		ws := monday.AddDate(0, 0, -7*i)
		a, b := ws.Unix(), ws.AddDate(0, 0, 7).Unix()
		var wNew, wActive, wMeas, wRet int
		for _, v := range vs {
			if !active(v, a, b) {
				continue
			}
			wActive++
			if v.first >= a {
				wNew++
				if n-v.first >= returnSecs {
					wMeas++
					if v.returned() {
						wRet++
					}
				}
			}
		}
		w := gin.H{"week": ws.Format(dayFmt), "new": wNew, "active": wActive, "returning": wActive - wNew, "measurable": wMeas, "returned": wRet}
		weeks = append(weeks, w)
	}

	// regulars who stopped coming
	gone := []gin.H{}
	for _, v := range vs {
		if len(v.days) >= regularDays && n-v.last >= goneSecs && n-v.last <= goneMaxSecs {
			gone = append(gone, gin.H{"key": v.key, "name_raw": v.name, "days": len(v.days), "playtime_s": v.play,
				"first_seen": v.first, "last_seen": v.last})
		}
	}
	sort.Slice(gone, func(i, j int) bool {
		return gone[i]["days"].(int) > gone[j]["days"].(int) ||
			gone[i]["days"].(int) == gone[j]["days"].(int) && gone[i]["last_seen"].(int64) > gone[j]["last_seen"].(int64)
	})
	if len(gone) > 50 {
		gone = gone[:50]
	}

	var rate any
	if measurable > 0 {
		rate = float64(returned) / float64(measurable)
	}
	return gin.H{"source": src, "new": newN, "active": activeN, "returning": back, "measurable": measurable, "returned": returned,
		"return_rate": rate, "new_players": newList, "weeks": weeks, "gone": gone,
		"rules": gin.H{"return_days": returnWithin, "regular_days": regularDays, "gone_days": goneAfter, "gone_max_days": goneMax}}, nil
}

// liveMatch: the rounds of the current match (same map and mode, no pause longer than matchGap), nil when the
// last round is older than that. A round still open is live for up to openRound: S&D can run without a time
// limit, so a long quiet round must not hide the score.
func liveMatch(db *sql.DB, id string, now int64) (gin.H, error) {
	rs, err := rows(db, `SELECT started_at, ended_at, COALESCE(map, '') AS map, COALESCE(gametype, '') AS gametype, winner,
		score_allies, score_axis, attackers FROM rounds WHERE server_id=? AND started_at>=? ORDER BY id DESC LIMIT 60`, id, now-6*3600)
	if err != nil || len(rs) == 0 {
		return nil, err
	}
	last := rs[0]
	end := last["started_at"].(int64)
	if e, ok := last["ended_at"].(int64); ok {
		end = e
	} else if now-end < openRound {
		end = now
	}
	if now-end > matchGap {
		return nil, nil
	}
	m := gin.H{"map": last["map"], "gametype": last["gametype"], "started_at": last["started_at"], "rounds": 0}
	finished := 0
	var scored map[string]any
	prev := last["started_at"].(int64)
	for _, r := range rs {
		if r["map"] != last["map"] || r["gametype"] != last["gametype"] {
			break
		}
		if e, ok := r["ended_at"].(int64); ok && prev-e > matchGap {
			break
		}
		prev = r["started_at"].(int64)
		m["started_at"] = prev
		if r["winner"] != nil {
			finished++
			if scored == nil {
				scored = r
			}
		}
	}
	m["rounds"] = finished
	if scored != nil {
		m["score_allies"], m["score_axis"], m["attackers"] = scored["score_allies"], scored["score_axis"], scored["attackers"]
	}
	return m, nil
}
