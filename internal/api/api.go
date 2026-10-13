// Package api serves the REST API, the SSE stream, /metrics and the embedded frontend.
package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"mime"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/push"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Hub fans out live events to SSE clients.
type Hub struct {
	mu   sync.Mutex
	subs map[chan []byte]struct{}
}

func NewHub() *Hub { return &Hub{subs: map[chan []byte]struct{}{}} }

func (h *Hub) Publish(b []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.subs {
		select {
		case c <- b:
		default: // slow client: drop event
		}
	}
}

func (h *Hub) sub() chan []byte {
	c := make(chan []byte, 16)
	h.mu.Lock()
	h.subs[c] = struct{}{}
	h.mu.Unlock()
	return c
}

func (h *Hub) unsub(c chan []byte) {
	h.mu.Lock()
	delete(h.subs, c)
	h.mu.Unlock()
}

var ranges = map[string]int64{"24h": 86400, "7d": 7 * 86400, "30d": 30 * 86400, "90d": 90 * 86400}

// span returns the [from, to) period: ?from=&to= (unix seconds) when given, else ?range= ending now.
func span(c *gin.Context, def string) (from, to int64) {
	now := time.Now().Unix()
	from, ferr := strconv.ParseInt(c.Query("from"), 10, 64)
	to, terr := strconv.ParseInt(c.Query("to"), 10, 64)
	if ferr == nil && terr == nil {
		to = min(to, now)
		return min(from, to-600), to // at least 10 min
	}
	r, ok := ranges[c.DefaultQuery("range", def)]
	if !ok {
		r = ranges[def]
	}
	return now - r, now
}

// occupancy returns the average and peak number of simultaneous connections [start, end) within [t, end).
// ponytail: called per bucket, O(buckets × connections); fine for thousands of connections per range.
func occupancy(conns [][2]int64, t, end int64) (avg float64, peak int) {
	var overlap int64
	type ev struct {
		at int64
		d  int
	}
	var evs []ev
	cur := 0
	for _, c := range conns {
		a, b := c[0], c[1]
		if a >= end || b <= t {
			continue
		}
		overlap += min(b, end) - max(a, t)
		if a <= t {
			cur++
		} else {
			evs = append(evs, ev{a, 1})
		}
		if b < end {
			evs = append(evs, ev{b, -1})
		}
	}
	sort.Slice(evs, func(i, j int) bool { // a leave and a join in the same second don't overlap
		return evs[i].at < evs[j].at || evs[i].at == evs[j].at && evs[i].d < evs[j].d
	})
	peak = cur
	for _, e := range evs {
		cur += e.d
		peak = max(peak, cur)
	}
	return float64(overlap) / float64(end-t), peak
}

// sameOrigin protects state-changing endpoints from other websites open in the browser:
// JSON only (forces a CORS preflight) and matching Origin (http, or https behind the tunnel). Without a login, also
// a local Host (blocks DNS rebinding); logged in, a rebinding page doesn't have the session cookie anyway.
func sameOrigin(c *gin.Context) {
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		host = c.Request.Host
	}
	origin := c.GetHeader("Origin")
	if !strings.HasPrefix(c.ContentType(), "application/json") ||
		(origin != "" && origin != "http://"+c.Request.Host && origin != "https://"+c.Request.Host) ||
		(!c.GetBool("loginmode") && host != "localhost" && host != "127.0.0.1" && host != "::1") {
		c.AbortWithStatusJSON(403, gin.H{"error": "forbidden"})
	}
}

// rows turns a query result into JSON-friendly maps.
func rows(db *sql.DB, q string, args ...any) ([]map[string]any, error) {
	r, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	cols, _ := r.Columns()
	out := []map[string]any{}
	for r.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := r.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			if b, ok := vals[i].([]byte); ok {
				vals[i] = string(b)
			}
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out, r.Err()
}

// publicEvent: live events a visitor may receive (not the console lines nor the chat).
func publicEvent(b []byte) bool {
	var ev struct{ Type string }
	json.Unmarshal(b, &ev)
	return ev.Type == "status" || ev.Type == "connection"
}

type Config struct {
	Store        *store.Store
	Hub          *Hub
	PollInterval time.Duration
	RecentLogs   func(serverID int64) []string // in-memory console tail, nil when no log is configured
	Rcon         RconOf                        // rcon of a server, nil when it has none (the list changes when servers are edited)
	// GameDirs lists mods/ and usermaps/ on the server (nil func: no FTP configured)
	GameDirs       func(ctx context.Context, serverID int64) (mods, usermaps []string, err error)
	ServerLoc      *time.Location
	User, Password string     // dashboard login, empty password = no login
	VPS            func() any // health of the machine the monitor runs on (disk, memory, load); nil = unknown
	TrustedProxies []string
	Servers        ServerManager // applies a server's settings saved from the dashboard (start, restart, stop)
	UI             fs.FS         // built interface (ui/dist): the page, maps/<map>.jpg, fonts; nil = not served
	Push           *push.Sender  // push notifications of the staff; nil = off
}

func init() {
	mime.AddExtensionType(".webmanifest", "application/manifest+json") // home screen app (Go doesn't know it)
}

func Router(cfg Config) *gin.Engine {
	st, hub, pollInterval, recentLogs, rcons, serverLoc, user, password :=
		cfg.Store, cfg.Hub, cfg.PollInterval, cfg.RecentLogs, cfg.Rcon, cfg.ServerLoc, cfg.User, cfg.Password
	db := st.DB
	r := gin.New()
	r.Use(gin.Recovery(), proxySecurity(r, cfg.TrustedProxies))
	if password != "" {
		authRoutes(r, db, user, password)
	} else { // local only, no login: the page asks who is connected
		r.Use(audit(db))
		r.GET("/api/me", func(c *gin.Context) {
			c.JSON(200, gin.H{"username": "", "role": roleAdmin, "admin": false, "noauth": true})
		})
	}

	r.GET("/api/audit", func(c *gin.Context) {
		rs, err := rows(db, `SELECT a.ts, a.username, a.action, a.detail, s.name AS server FROM admin_log a
			LEFT JOIN servers s ON s.id=a.server_id WHERE ? OR s.owner=? ORDER BY a.id DESC LIMIT 300`, super(c), c.GetString("user"))
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"log": rs})
	})

	// send runs queries and writes {key: rows}; aborts with 500 on the first error.
	send := func(c *gin.Context, parts map[string]func() (any, error)) {
		out := gin.H{}
		for k, f := range parts {
			v, err := f()
			if err != nil {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
			out[k] = v
		}
		c.JSON(200, out)
	}
	q := func(sql string, args ...any) func() (any, error) {
		return func() (any, error) { return rows(db, sql, args...) }
	}

	serverConfRoutes(r, st, cfg.Servers)
	observabilityRoutes(r, st)

	// accounts only (not in visitorRoutes)
	r.GET("/api/vps", func(c *gin.Context) {
		if cfg.VPS == nil {
			c.JSON(200, gin.H{})
			return
		}
		c.JSON(200, cfg.VPS())
	})

	r.GET("/api/health", func(c *gin.Context) {
		if err := db.Ping(); err != nil {
			c.JSON(503, gin.H{"status": "db down"})
			return
		}
		c.JSON(200, gin.H{"status": "ok"})
	})

	r.GET("/api/servers", func(c *gin.Context) {
		send(c, map[string]func() (any, error){"poll_interval_s": func() (any, error) { return pollInterval.Seconds(), nil }, "servers": q(`
			SELECT s.id, s.name, s.host, s.port, s.enabled, s.accent, s.logo_ts, sn.ts, sn.online, sn.latency_ms, sn.hostname, sn.map, sn.gametype, sn.players, sn.max_players,
			  EXISTS(SELECT 1 FROM match_scores WHERE server_id=s.id) OR EXISTS(SELECT 1 FROM map_votes WHERE server_id=s.id)
			  OR EXISTS(SELECT 1 FROM kills WHERE server_id=s.id AND tags IS NOT NULL) AS mod -- the pm_sniper log lines were seen: show their sections
			FROM servers s LEFT JOIN snapshots sn ON sn.id = (SELECT MAX(id) FROM snapshots WHERE server_id=s.id) ORDER BY s.id`)})
	})

	g := r.Group("/api/servers/:id")
	g.Use(func(c *gin.Context) {
		if _, err := strconv.Atoi(c.Param("id")); err != nil {
			c.AbortWithStatusJSON(400, gin.H{"error": "bad id"})
		}
	})

	g.GET("/status", func(c *gin.Context) {
		id := c.Param("id")
		send(c, map[string]func() (any, error){
			"snapshot": q(`SELECT * FROM snapshots WHERE server_id=? ORDER BY id DESC LIMIT 1`, id),
			"last_ok":  q(`SELECT MAX(ts) AS ts FROM snapshots WHERE server_id=? AND online=1`, id),
			"players": q(`SELECT ps.name_raw, ps.name_clean, ps.score, ps.ping, ses.started_at FROM player_sightings ps
				LEFT JOIN player_sessions ses ON ses.server_id=? AND ses.name_raw=ps.name_raw AND ses.open=1
				WHERE ps.snapshot_id=(SELECT MAX(id) FROM snapshots WHERE server_id=? AND online=1)
				  AND (SELECT online FROM snapshots WHERE server_id=? ORDER BY id DESC LIMIT 1)=1
				ORDER BY ps.score DESC`, id, id, id),
			"vars": func() (any, error) {
				var js string
				err := db.QueryRow(`SELECT vars FROM cvar_history WHERE server_id=? ORDER BY ts DESC LIMIT 1`, id).Scan(&js)
				if err == sql.ErrNoRows {
					return nil, nil
				}
				// show the server's own dates (g_mapStartTime...) in local time; stored raw
				var vars map[string]string
				if json.Unmarshal([]byte(js), &vars) == nil {
					for k := range vars {
						if query.SecretField(k) {
							delete(vars, k)
						}
					}
					clean, _ := json.Marshal(vars)
					js = string(clean)
				}
				return json.RawMessage(query.LocalizeDates(js, serverLoc, time.Local)), err
			},
		})
	})

	g.GET("/history", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "24h")
		bucket := max((to-from)/288, 60) // ~288 points per range (5 min for 24h)
		now := time.Now().Unix()
		snaps, err := rows(db, `SELECT (ts/?1)*?1 AS t, AVG(players) AS avg_players, MAX(players) AS max_players, AVG(latency_ms) AS latency_ms
			FROM snapshots WHERE server_id=?2 AND ts>=?3 AND ts<?4 AND online=1 GROUP BY t`, bucket, id, from, to)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		connRows, err := rows(db, `SELECT joined_at AS a, IIF(open=1, ?1, left_at) AS b FROM connections
			WHERE server_id=?2 AND (open=1 OR left_at IS NOT NULL) AND joined_at<?4 AND IIF(open=1, ?1, left_at)>=?3`, now, id, from, to)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		conns := make([][2]int64, len(connRows))
		for i, r := range connRows {
			conns[i] = [2]int64{r["a"].(int64), r["b"].(int64)}
		}
		byT := map[int64]map[string]any{}
		for _, r := range snaps {
			byT[r["t"].(int64)] = r
		}
		// Real connections (enterleave.log) give exact player counts, also before monitoring started.
		points := []map[string]any{}
		for t := from / bucket * bucket; t < to; t += bucket {
			p := map[string]any{"t": t, "avg_players": nil, "max_players": nil, "latency_ms": nil}
			if r := byT[t]; r != nil {
				p["avg_players"], p["max_players"], p["latency_ms"] = r["avg_players"], r["max_players"], r["latency_ms"]
			}
			if len(conns) > 0 {
				if end := min(t+bucket, to); end > t {
					p["avg_players"], p["max_players"] = occupancy(conns, t, end)
				}
			}
			points = append(points, p)
		}
		send(c, map[string]func() (any, error){
			"bucket": func() (any, error) { return bucket, nil },
			"source": func() (any, error) { return map[bool]string{true: "enterleave", false: "udp"}[len(conns) > 0], nil },
			"points": func() (any, error) { return points, nil },
			"maps": q(`SELECT ts, map, gametype FROM (
				SELECT ts, map, gametype, LAG(map) OVER (ORDER BY ts) AS prev FROM snapshots
				WHERE server_id=? AND ts>=? AND ts<? AND online=1) WHERE prev IS NULL OR prev<>map ORDER BY ts`, id, from, to),
			"from": func() (any, error) { return from, nil },
			"to":   func() (any, error) { return to, nil },
		})
	})

	g.GET("/uptime", func(c *gin.Context) {
		id := c.Param("id")
		now := time.Now().Unix()
		pct := func(span int64) func() (any, error) {
			return func() (any, error) {
				var v sql.NullFloat64
				err := db.QueryRow(`SELECT AVG(online)*100 FROM snapshots WHERE server_id=? AND ts>=?`, id, now-span).Scan(&v)
				if !v.Valid {
					return nil, err
				}
				return v.Float64, err
			}
		}
		send(c, map[string]func() (any, error){
			"24h": pct(ranges["24h"]), "7d": pct(ranges["7d"]), "30d": pct(ranges["30d"]),
			// the console lines stay private (player names, IPs)
			// availability of each of the last 14 days (local time), for the day-by-day bar
			"days": q(`SELECT date(ts, 'unixepoch', 'localtime') AS day, AVG(online)*100 AS pct FROM snapshots
				WHERE server_id=? AND ts>=? GROUP BY day ORDER BY day`, id, now-14*86400),
			"outages": q(`SELECT started_at, ended_at, COALESCE(ended_at, ?) - started_at AS duration_s, reason,
				IIF(?, NULL, console) AS console, COALESCE(restart, 0) AS restart, COALESCE(clean, 0) AS clean
				FROM outages WHERE server_id=? AND COALESCE(ended_at, ?)>=? ORDER BY started_at DESC LIMIT 200`,
				now, role(c) == roleVisitor, id, now, now-ranges["30d"]),
		})
	})

	// Stats come from enterleave.log connections (real times + GUID) when available,
	// else from UDP sessions (name only, merged through the GUIDs seen in games_mp.log).
	g.GET("/players", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		var hasConn bool
		if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM connections WHERE server_id=?)`, id).Scan(&hasConn); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		const common = `
			  CASE WHEN s.guid IS NULL THEN (SELECT COUNT(*) FROM chat_messages m WHERE m.server_id=?1 AND m.name_clean=s.name_clean)
			       ELSE (SELECT COUNT(*) FROM chat_messages m WHERE m.server_id=?1 AND m.guid=s.guid) END AS messages,
			  (SELECT group_concat(name_clean, ', ') FROM player_names pn WHERE pn.server_id=?1 AND pn.guid=s.guid) AS aliases,
			  (SELECT name_raw FROM s s2 WHERE s2.pid=s.pid ORDER BY last_seen DESC LIMIT 1) AS name_raw,
			  (SELECT name_clean FROM s s2 WHERE s2.pid=s.pid ORDER BY last_seen DESC LIMIT 1) AS name_clean,
			  (SELECT level FROM player_profiles pp WHERE pp.server_id=?1 AND pp.key=s.pid ORDER BY ts DESC LIMIT 1) AS level,
			  (SELECT prestige FROM player_profiles pp WHERE pp.server_id=?1 AND pp.key=s.pid ORDER BY ts DESC LIMIT 1) AS prestige,
			  (SELECT SUM(kills) FROM daily_combat dc WHERE dc.server_id=?1 AND dc.key=s.pid) AS kills,
			  (SELECT SUM(deaths) FROM daily_combat dc WHERE dc.server_id=?1 AND dc.key=s.pid) AS deaths
			FROM s GROUP BY pid ORDER BY playtime_s DESC LIMIT 500`
		ipCol := `(SELECT ip FROM s s2 WHERE s2.pid=s.pid ORDER BY joined_at DESC LIMIT 1)`
		if role(c) == roleVisitor {
			ipCol = `NULL`
		}
		if hasConn {
			send(c, map[string]func() (any, error){"source": func() (any, error) { return "enterleave", nil }, "players": q(`
				WITH s AS (SELECT *, COALESCE(left_at, joined_at) AS last_seen, COALESCE(guid, 'n:'||name_clean) AS pid
				           FROM connections WHERE server_id=?1 AND IIF(open=1, ?3, COALESCE(left_at, joined_at)) >= ?2 AND joined_at < ?4)
				SELECT s.guid, s.pid AS key, MAX(open) AS online, COUNT(*) AS sessions, MIN(joined_at) AS first_seen,
				  MAX(CASE WHEN open=1 THEN ?3 ELSE last_seen END) AS last_seen,
				  -- time played inside [from, to); unknown ends (crash before monitoring) count 0
				  SUM(MAX(0, MIN(COALESCE(left_at, IIF(open=1, ?3, joined_at)), ?4) - MAX(joined_at, ?2))) AS playtime_s,
				  (SELECT MAX(best_score) FROM player_sessions ps WHERE ps.server_id=?1 AND ps.name_clean IN (SELECT name_clean FROM s s2 WHERE s2.pid=s.pid)) AS best_score,
				  `+ipCol+` AS ip,`+common, id, from, time.Now().Unix(), to)})
			return
		}
		send(c, map[string]func() (any, error){"source": func() (any, error) { return "udp", nil }, "players": q(`
			WITH ng AS (SELECT name_clean, guid, MAX(last_seen) FROM player_names WHERE server_id=?1 GROUP BY name_clean),
			s AS (SELECT p.*, ng.guid, COALESCE(ng.guid, 'n:'||p.name_clean) AS pid
			      FROM player_sessions p LEFT JOIN ng USING(name_clean) WHERE p.server_id=?1 AND p.last_seen>=?2 AND p.started_at<?3)
			SELECT s.guid, s.pid AS key, MAX(open) AS online, COUNT(*) AS sessions, MIN(started_at) AS first_seen, MAX(last_seen) AS last_seen,
			  SUM(last_seen-started_at) AS playtime_s, MAX(best_score) AS best_score, NULL AS ip,`+common, id, from, to)})
	})

	// Every name each GUID used (and the IPs it connected from), players with several names first.
	g.GET("/aliases", func(c *gin.Context) {
		send(c, map[string]func() (any, error){"aliases": q(`
			SELECT guid, name_raw, name_clean, first_seen, last_seen,
			  (SELECT COUNT(*) FROM player_names p2 WHERE p2.server_id=p.server_id AND p2.guid=p.guid) AS names,
			  (SELECT group_concat(DISTINCT ip) FROM connections c WHERE c.server_id=p.server_id AND c.guid=p.guid) AS ips
			FROM player_names p WHERE server_id=? ORDER BY names DESC, guid, last_seen DESC`, c.Param("id"))})
	})

	g.GET("/maps", func(c *gin.Context) {
		id := c.Param("id")
		from, to := span(c, "30d")
		send(c, map[string]func() (any, error){"maps": q(`
			SELECT map, COUNT(*) AS samples, AVG(players) AS avg_players, MAX(players) AS max_players
			FROM snapshots WHERE server_id=? AND ts>=? AND ts<? AND online=1 GROUP BY map ORDER BY samples DESC`, id, from, to)})
	})

	g.GET("/cvars", func(c *gin.Context) {
		changes, err := rows(db, `SELECT ts,vars FROM cvar_history WHERE server_id=? ORDER BY ts DESC LIMIT 50`, c.Param("id"))
		if err != nil {
			c.Status(500)
			return
		}
		for _, change := range changes {
			raw, _ := change["vars"].(string)
			vars := map[string]string{}
			if json.Unmarshal([]byte(raw), &vars) != nil {
				vars = map[string]string{}
			}
			for key := range vars {
				if query.SecretField(key) {
					delete(vars, key)
				}
			}
			clean, _ := json.Marshal(vars)
			change["vars"] = string(clean)
		}
		c.JSON(200, gin.H{"changes": changes})
	})

	g.GET("/logs", func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		var lines []string
		if recentLogs != nil {
			lines = recentLogs(id)
		}
		c.JSON(200, gin.H{"enabled": lines != nil, "lines": lines})
	})

	// ?guid=<guid>, or ?name=<part of any of the player's names> (case-insensitive), ?q=<text> to search, newest first.
	g.GET("/chat", func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
		send(c, map[string]func() (any, error){"messages": q(`SELECT ts, guid, name_raw, name_clean, team, message FROM chat_messages
			WHERE server_id=?1 AND (?2='' OR name_clean LIKE '%'||?2||'%'
			  OR guid IN (SELECT guid FROM player_names WHERE server_id=?1 AND name_clean LIKE '%'||?2||'%')) AND (?3='' OR message LIKE '%'||?3||'%') AND (?5='' OR guid=?5)
			ORDER BY ts DESC, id DESC LIMIT ?4`,
			c.Param("id"), c.Query("name"), c.Query("q"), min(max(limit, 1), 2000), c.Query("guid"))})
	})

	g.GET("/rcon", func(c *gin.Context) {
		id, _ := strconv.ParseInt(c.Param("id"), 10, 64)
		c.JSON(200, gin.H{"enabled": rcons.of(id) != nil})
	})

	adminRoutes(g, st, rcons)
	b3Routes(g, st, rcons)
	featureRoutes(g, st, rcons)
	catalogRoutes(g, cfg.UI, cfg.GameDirs)
	statsRoutes(g, db)
	matchRoutes(g, db)
	experienceRoutes(g, db)
	cardRoutes(g, db)
	weekRoutes(g, db)
	insightRoutes(g, db)
	liveRoutes(g, db)
	inboxRoutes(g, st)
	brandRoutes(g, db)
	meRoutes(r, st)
	pushRoutes(r, st, cfg.Push)

	r.GET("/api/events", func(c *gin.Context) {
		ch := hub.sub()
		defer hub.unsub(ch)
		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		ping := time.NewTicker(25 * time.Second)
		defer ping.Stop()
		c.Writer.WriteHeader(200)
		c.Writer.Flush()
		for {
			select {
			case <-c.Request.Context().Done():
				return
			case b := <-ch:
				if !eventAllowed(db, c, b) {
					continue
				}
				fmt.Fprintf(c.Writer, "data: %s\n\n", b)
			case <-ping.C:
				fmt.Fprint(c.Writer, ": ping\n\n")
			}
			c.Writer.Flush()
		}
	})

	r.GET("/metrics", func(c *gin.Context) {
		rs, err := rows(db, `SELECT s.name, sn.online, sn.latency_ms, sn.players, sn.max_players FROM servers s
			JOIN snapshots sn ON sn.id = (SELECT MAX(id) FROM snapshots WHERE server_id=s.id)`)
		if err != nil {
			c.String(500, err.Error())
			return
		}
		w := c.Writer
		c.Header("Content-Type", "text/plain; version=0.0.4")
		logMetrics(c, st)
		for _, m := range []struct{ name, col, help string }{
			{"cod4_up", "online", "1 if the server answered the last poll"},
			{"cod4_latency_ms", "latency_ms", "Query round-trip time"},
			{"cod4_players", "players", "Connected players"},
			{"cod4_max_players", "max_players", "Player slots"},
		} {
			fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n", m.name, m.help, m.name)
			for _, row := range rs {
				if v := row[m.col]; v != nil {
					fmt.Fprintf(w, "%s{server=%q} %v\n", m.name, row["name"], v)
				}
			}
		}
	})

	if cfg.UI != nil {
		r.NoRoute(appHandler(cfg.UI))
	}
	return r
}
