package api

import (
	"database/sql"
	"sort"
	"strings"
)

// Rounds and winners for any mod. The stock game scripts write no round result in games_mp.log (their logString lines
// are dropped by the server), only the kills: K;victim guid;slot;team;name;killer guid;slot;team;name;... So, when the
// mod doesn't write its own result lines (RE, ME):
//   - S&D (no respawn in a round): a round ends when a team is eliminated, and a player killed a second time means a
//     new round has started;
//   - the winner of a round is the team left standing: every player of the other team was killed (the players of a
//     round are those seen in the kills within a few minutes of its start, with the team of their nearest appearance).
//     A bomb planted then defused (P_D line, written by Promod and many mods) goes to the defuser's team, a bomb planted
//     and not defused to the planter's team. Otherwise the winner stays unknown;
//   - team deathmatch: the team with the most kills wins the match.
// These results are estimated: a player who neither killed nor died around a round is unknown to the monitor.

type roundKill struct {
	ID, Round, TS      int64
	Killer, KillerName string // "" for the world (fall, suicide by explosion…)
	KillerTeam         string
	Victim, VictimTeam string
	TK                 bool
}

type roundEvent struct {
	Round, TS int64
	Type, Key string
	Text      string // type:name:value, as the rounds of the match page show them
}

// playedRound: a round as the match page shows it. ID is the round's id in the database, or a negative id for the
// rounds found inside one (S&D logged as one round per map).
type playedRound struct {
	ID          int64  `json:"id"`
	Start       int64  `json:"started_at"`
	End         int64  `json:"ended_at"`
	Kills       int    `json:"kills"`
	Winner      string `json:"winner,omitempty"`
	ScoreAllies *int   `json:"score_allies"`
	ScoreAxis   *int   `json:"score_axis"`
	Estimated   bool   `json:"estimated,omitempty"`
	FirstBlood  string `json:"first_blood,omitempty"`
	TopKiller   string `json:"top_killer,omitempty"`
	TopKills    int    `json:"top_kills,omitempty"`
	Events      string `json:"events,omitempty"`
	dbRound     int64
	roster      map[string]string // the players of the round and their team (inferRounds)
}

// eliminationMode: game types without respawn during a round.
func eliminationMode(gametype string) bool {
	gt := strings.ToLower(gametype)
	return gt == "sd" || strings.HasSuffix(gt, "_sd") || strings.HasPrefix(gt, "sd_")
}

func teamSide(t string) bool { return t == "allies" || t == "axis" }

func other(t string) string {
	if t == "allies" {
		return "axis"
	}
	return "allies"
}

// inferRounds splits the kills of a match into rounds and finds their winners. The database rounds keep the result the
// mod wrote (RE line); when it writes one, they are used as they are.
func inferRounds(gametype string, dbRounds []playedRound, kills []roundKill, events []roundEvent) []playedRound {
	officialResults := false
	for _, r := range dbRounds {
		if r.Winner != "" {
			officialResults = true
		}
	}
	split := eliminationMode(gametype) && !officialResults
	byRound := map[int64]*playedRound{}
	for i := range dbRounds {
		dbRounds[i].dbRound = dbRounds[i].ID
		byRound[dbRounds[i].ID] = &dbRounds[i]
	}
	sort.SliceStable(kills, func(i, j int) bool {
		return kills[i].TS < kills[j].TS || kills[i].TS == kills[j].TS && kills[i].ID < kills[j].ID
	})

	// where each player was seen, and with which team: the players of a round are those seen in the kills within
	// a few minutes of its start, with the team of their nearest appearance
	type sighting struct {
		ts   int64
		team string
	}
	appear := map[string][]sighting{}
	for _, k := range kills {
		if teamSide(k.VictimTeam) {
			appear[k.Victim] = append(appear[k.Victim], sighting{k.TS, k.VictimTeam})
		}
		if k.Killer != "" && teamSide(k.KillerTeam) {
			appear[k.Killer] = append(appear[k.Killer], sighting{k.TS, k.KillerTeam})
		}
	}
	const window = 180
	roster := func(at int64) map[string]string {
		team := map[string]string{}
		for key, list := range appear {
			best, gap := "", int64(window+1)
			for _, a := range list {
				if d := max(a.ts-at, at-a.ts); d < gap {
					best, gap = a.team, d
				}
			}
			if best != "" {
				team[key] = best
			}
		}
		return team
	}

	// the rounds, in time order: a new one with each database round, and (S&D) after a team was eliminated or when a
	// player dies a second time
	type found struct {
		kills  []roundKill
		team   map[string]string // the players of the round
		winner string            // the team left standing
	}
	var rounds []*found
	var dead map[string]bool
	for _, k := range kills {
		cur := (*found)(nil)
		if n := len(rounds); n > 0 {
			cur = rounds[n-1]
		}
		if cur == nil || k.Round != cur.kills[0].Round || split && (dead[k.Victim] || cur.winner != "") {
			cur = &found{}
			if split {
				cur.team = roster(k.TS)
			}
			rounds = append(rounds, cur)
			dead = map[string]bool{}
		}
		dead[k.Victim] = true
		cur.kills = append(cur.kills, k)
		if !split {
			continue
		}
		if teamSide(k.VictimTeam) { // seen in this round: its team now
			cur.team[k.Victim] = k.VictimTeam
		}
		if k.Killer != "" && teamSide(k.KillerTeam) {
			cur.team[k.Killer] = k.KillerTeam
		}
		alive, members := map[string]int{}, map[string]int{}
		for key, t := range cur.team {
			members[t]++
			if !dead[key] {
				alive[t]++
			}
		}
		for _, t := range []string{"allies", "axis"} {
			if members[t] > 0 && alive[t] == 0 && alive[other(t)] > 0 {
				cur.winner = other(t)
			}
		}
	}

	// the rounds as the match page shows them
	team := map[string]string{} // latest team of each player, for the bomb events
	var out []playedRound
	for _, f := range rounds {
		first := f.kills[0]
		r := playedRound{ID: first.Round, Start: first.TS, dbRound: first.Round}
		if base := byRound[first.Round]; base != nil {
			r = *base
			if len(out) > 0 && out[len(out)-1].dbRound == first.Round { // a round found inside the database round
				r.ID, r.Start, r.Winner, r.ScoreAllies, r.ScoreAxis = -first.ID, first.TS, "", nil, nil
			}
		}
		r.Kills, r.FirstBlood, r.TopKiller, r.Events, r.End = 0, "", "", "", r.Start
		r.roster = f.team
		killsBy, names := map[string]int{}, map[string]string{}
		for _, k := range f.kills {
			r.Kills++
			r.End = max(r.End, k.TS)
			if k.Killer != "" && !k.TK && k.Killer != k.Victim {
				if r.FirstBlood == "" {
					r.FirstBlood = k.KillerName
				}
				killsBy[k.Killer]++
				names[k.Killer] = k.KillerName
			}
			if teamSide(k.VictimTeam) {
				team[k.Victim] = k.VictimTeam
			}
			if k.Killer != "" && teamSide(k.KillerTeam) {
				team[k.Killer] = k.KillerTeam
			}
		}
		top, n := "", 0
		for k, v := range killsBy {
			if v > n || v == n && k < top {
				top, n = k, v
			}
		}
		r.TopKiller, r.TopKills = names[top], n
		if f.winner != "" {
			r.Winner, r.Estimated = f.winner, true
		}
		out = append(out, r)
	}

	// rounds without kills (empty server) stay as they are, then everything in time order
	seen := map[int64]bool{}
	for _, r := range out {
		seen[r.dbRound] = true
	}
	for _, r := range dbRounds {
		if !seen[r.ID] {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Start < out[j].Start })

	// the objective events of each round (by time), and the bomb deciding the winner
	for i := range out {
		end := int64(1 << 62)
		if i+1 < len(out) && out[i+1].dbRound == out[i].dbRound {
			end = out[i+1].Start
		}
		var texts []string
		planter, defuser := "", ""
		for _, e := range events {
			if e.Round != out[i].dbRound || e.TS >= end || out[i].ID < 0 && e.TS < out[i].Start {
				continue
			}
			texts = append(texts, e.Text)
			switch e.Type {
			case "plant":
				planter = e.Key
			case "defuse":
				defuser = e.Key
			}
		}
		out[i].Events = strings.Join(texts, "|")
		teamOf := func(key string) string {
			if t := out[i].roster[key]; t != "" {
				return t
			}
			return team[key]
		}
		if split {
			switch {
			case defuser != "" && teamSide(teamOf(defuser)):
				out[i].Winner, out[i].Estimated = teamOf(defuser), true
			case planter != "" && teamSide(teamOf(planter)):
				out[i].Winner, out[i].Estimated = teamOf(planter), true
			}
		}
	}

	// running score from the estimated winners
	if split {
		a, x := 0, 0
		for i := range out {
			switch out[i].Winner {
			case "allies":
				a++
			case "axis":
				x++
			}
			sa, sx := a, x
			out[i].ScoreAllies, out[i].ScoreAxis = &sa, &sx
		}
	}
	return out
}

// estimatedResult: winner and score of a match without the mod's result lines, from its rounds (S&D) or its kills
// (team deathmatch). ok=false: unknown.
func estimatedResult(gametype string, rounds []playedRound, kills []roundKill) (winner string, allies, axis int, ok bool) {
	if eliminationMode(gametype) {
		known := false
		for _, r := range rounds {
			switch r.Winner {
			case "allies":
				allies++
			case "axis":
				axis++
			}
			known = known || r.Winner != ""
		}
		if !known {
			return "", 0, 0, false
		}
	} else if gt := strings.ToLower(gametype); gt == "war" || gt == "tdm" {
		for _, k := range kills {
			if k.Killer == "" || k.TK || k.Killer == k.Victim {
				continue
			}
			switch k.KillerTeam {
			case "allies":
				allies++
			case "axis":
				axis++
			}
		}
		if allies+axis == 0 {
			return "", 0, 0, false
		}
	} else {
		return "", 0, 0, false
	}
	switch {
	case allies > axis:
		winner = "allies"
	case axis > allies:
		winner = "axis"
	default:
		winner = "tie"
	}
	return winner, allies, axis, true
}

// matchKills: the kills and objective events of the given database rounds, for inferRounds.
func matchKills(db *sql.DB, server string, roundIDs []int64) ([]roundKill, []roundEvent, error) {
	if len(roundIDs) == 0 {
		return nil, nil, nil
	}
	in, args := placeholders(roundIDs)
	rs, err := db.Query(`SELECT id, round_id, ts, COALESCE(killer_key, ''), COALESCE(killer_name, ''), COALESCE(killer_team, ''),
		victim_key, COALESCE(victim_team, ''), teamkill FROM kills WHERE server_id=? AND round_id IN (`+in+`)`, append([]any{server}, args...)...)
	if err != nil {
		return nil, nil, err
	}
	var kills []roundKill
	for rs.Next() {
		var k roundKill
		if err := rs.Scan(&k.ID, &k.Round, &k.TS, &k.Killer, &k.KillerName, &k.KillerTeam, &k.Victim, &k.VictimTeam, &k.TK); err != nil {
			rs.Close()
			return nil, nil, err
		}
		kills = append(kills, k)
	}
	rs.Close()
	if err := rs.Err(); err != nil {
		return nil, nil, err
	}
	es, err := db.Query(`SELECT round_id, ts, type, COALESCE(key, ''), type||':'||COALESCE(name_raw, '')||':'||COALESCE(value, 0)
		FROM game_events WHERE round_id IN (`+in+`) ORDER BY ts, id`, args...)
	if err != nil {
		return nil, nil, err
	}
	defer es.Close()
	var events []roundEvent
	for es.Next() {
		var e roundEvent
		if err := es.Scan(&e.Round, &e.TS, &e.Type, &e.Key, &e.Text); err != nil {
			return nil, nil, err
		}
		events = append(events, e)
	}
	return kills, events, es.Err()
}
