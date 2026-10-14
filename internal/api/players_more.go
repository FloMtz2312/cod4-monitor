package api

import (
	"strconv"
	"strings"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// morePlayers completes the player list (connections of enterleave.log, or sessions seen by the queries) with the
// players they miss: GUIDs seen in games_mp.log (before enterleave.log was kept, for example) and the players B3
// knows. They have no playtime; "source" says where they come from ("log" or "b3").
func morePlayers(st *store.Store, id string, from, to int64, visitor bool, players []map[string]any) ([]map[string]any, error) {
	known := map[string]bool{}
	for _, p := range players {
		if k, ok := p["key"].(string); ok {
			known[k] = true
		}
		if g, ok := p["guid"].(string); ok && g != "" {
			known[g] = true
		}
	}
	add := func(p map[string]any) {
		k, _ := p["key"].(string)
		if k == "" || known[k] {
			return
		}
		known[k] = true
		p["online"], p["playtime_s"], p["best_score"] = int64(0), nil, nil
		players = append(players, p)
	}

	logged, err := rows(st.DB, `SELECT guid, guid AS key, MIN(first_seen) AS first_seen, MAX(last_seen) AS last_seen, NULL AS sessions, NULL AS ip,
		(SELECT name_raw FROM player_names p2 WHERE p2.server_id=?1 AND p2.guid=p.guid ORDER BY last_seen DESC LIMIT 1) AS name_raw,
		(SELECT name_clean FROM player_names p2 WHERE p2.server_id=?1 AND p2.guid=p.guid ORDER BY last_seen DESC LIMIT 1) AS name_clean,
		group_concat(name_clean, ', ') AS aliases,
		(SELECT COUNT(*) FROM chat_messages m WHERE m.server_id=?1 AND m.guid=p.guid) AS messages,
		(SELECT level FROM player_profiles pp WHERE pp.server_id=?1 AND pp.key=p.guid ORDER BY ts DESC LIMIT 1) AS level,
		(SELECT prestige FROM player_profiles pp WHERE pp.server_id=?1 AND pp.key=p.guid ORDER BY ts DESC LIMIT 1) AS prestige,
		(SELECT SUM(kills) FROM daily_combat dc WHERE dc.server_id=?1 AND dc.key=p.guid) AS kills,
		(SELECT SUM(deaths) FROM daily_combat dc WHERE dc.server_id=?1 AND dc.key=p.guid) AS deaths,
		'log' AS source
		FROM player_names p WHERE server_id=?1 AND last_seen>=?2 AND first_seen<?3 GROUP BY guid ORDER BY last_seen DESC LIMIT 500`, id, from, to)
	if err != nil {
		return nil, err
	}
	for _, p := range logged {
		add(p)
	}

	// B3: the players it saw in the period (its first and last visit, its count of connections)
	sid, _ := strconv.ParseInt(id, 10, 64)
	conf, found, err := st.ServerConf(sid)
	if err != nil || !found || conf.Logs.B3DB == "" {
		return players, err
	}
	d, err := b3.Open(conf.Logs.B3DB)
	if err != nil {
		return players, nil // B3 unreachable: the list without it
	}
	defer d.Close()
	cs, err := d.Seen(from, to, 500)
	if err != nil {
		return players, nil
	}
	var keys []string
	for _, c := range cs {
		if !known[c.GUID] {
			keys = append(keys, c.GUID)
		}
	}
	stats := map[string]map[string]any{}
	if len(keys) > 0 {
		args := []any{id}
		for _, k := range keys {
			args = append(args, k)
		}
		rs, err := rows(st.DB, `SELECT dc.key, SUM(kills) AS kills, SUM(deaths) AS deaths,
			(SELECT level FROM player_profiles pp WHERE pp.server_id=?1 AND pp.key=dc.key ORDER BY ts DESC LIMIT 1) AS level,
			(SELECT prestige FROM player_profiles pp WHERE pp.server_id=?1 AND pp.key=dc.key ORDER BY ts DESC LIMIT 1) AS prestige
			FROM daily_combat dc WHERE dc.server_id=?1 AND dc.key IN (?`+strings.Repeat(",?", len(keys)-1)+`) GROUP BY dc.key`, args...)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			stats[r["key"].(string)] = r
		}
	}
	for _, c := range cs {
		p := map[string]any{"guid": c.GUID, "key": c.GUID, "name_raw": c.Name, "name_clean": query.StripColors(c.Name),
			"first_seen": c.TimeAdd, "last_seen": c.TimeEdit, "sessions": c.Connections, "messages": nil, "aliases": nil, "source": "b3"}
		if !visitor && c.IP != "" {
			p["ip"] = c.IP
		}
		if s := stats[c.GUID]; s != nil {
			p["kills"], p["deaths"], p["level"], p["prestige"] = s["kills"], s["deaths"], s["level"], s["prestige"]
		}
		add(p)
	}
	return players, nil
}
