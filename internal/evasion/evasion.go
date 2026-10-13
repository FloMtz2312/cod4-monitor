// Package evasion looks for banned players back under another GUID (GUID spoofer): every player active since a
// ban is compared with the banned player, and each matching clue adds points. Nothing is banned automatically.
package evasion

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Points of each clue. Shown from Min, live alert on the dashboard from Alert.
const (
	ptIP       = 60 // same IP as the banned player
	ptSteam    = 60 // same Steam id
	ptProfile  = 40 // same career profile (prestige, career kills just above the banned player's)
	ptName     = 25 // same name once colours, case and l33t are removed
	ptSubnet   = 20 // same /24 network (same ISP area), when the IP differs
	ptVeteran  = 20 // GUID first seen after the ban, already with a big career (level 10+ or a prestige)
	ptSettings = 15 // same non-default network settings (rate / snaps / cl_maxpackets)
	ptNameNear = 15 // name 1-2 letters away
	ptStyle    = 10 // similar headshot %, noscope % and kill distance
	ptNew      = 10 // GUID first seen after the ban (only on top of another clue)

	Min   = 30
	Alert = 50
)

type Reason struct {
	Code   string `json:"code"` // ip steam profile name subnet veteran settings name_near style new
	Points int    `json:"points"`
	Detail string `json:"detail"`
}

type Suspect struct {
	Key       string   `json:"key"`
	NameRaw   string   `json:"name_raw"`
	FirstSeen int64    `json:"first_seen"`
	LastSeen  int64    `json:"last_seen"`
	ConnID    int64    `json:"conn_id"`
	BanKey    string   `json:"ban_guid"`
	BanName   string   `json:"ban_name"`
	BanTS     int64    `json:"ban_ts"`
	Duration  string   `json:"duration"`
	Score     int      `json:"score"`
	Reasons   []Reason `json:"reasons"`
}

type player struct {
	key, name, clean   string
	first, last, conn  int64
	ips, steam         map[string]bool
	level, prestige, k int // latest career profile (PI line), k = career kills; level 0 = unknown
	settings           string
	kills, hs, ns      int
	dist               sql.NullFloat64
}

// Find returns the suspects active since `since` (unix), best ban match per player, highest score first.
func Find(db *sql.DB, server, since int64) ([]Suspect, error) {
	players, err := load(db, server)
	if err != nil {
		return nil, err
	}
	bans, err := db.Query(`SELECT ts, COALESCE(guid, ''), COALESCE(name_raw, ''), COALESCE(ip, ''), COALESCE(duration, '') FROM bans WHERE server_id=?`, server)
	if err != nil {
		return nil, err
	}
	defer bans.Close()
	best := map[string]*Suspect{}
	for bans.Next() {
		var ts int64
		var guid, name, ip, duration string
		if err := bans.Scan(&ts, &guid, &name, &ip, &duration); err != nil {
			return nil, err
		}
		key := store.PlayerKey(guid, query.StripColors(name))
		banned := players[key]
		if banned == nil {
			banned = &player{key: key, name: name, clean: query.StripColors(name), ips: map[string]bool{}, steam: map[string]bool{}}
		}
		if ip != "" {
			banned.ips[ip] = true
		}
		for _, p := range players {
			if p.key == key || p.last <= ts || p.last < since {
				continue
			}
			s := Score(p, banned, ts)
			if s.Score < Min {
				continue
			}
			if b := best[p.key]; b == nil || s.Score > b.Score {
				s.BanKey, s.BanName, s.BanTS, s.Duration = guid, name, ts, duration
				best[p.key] = &s
			}
		}
	}
	if err := bans.Err(); err != nil {
		return nil, err
	}
	out := []Suspect{}
	for _, s := range best {
		out = append(out, *s)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Score > out[j].Score || out[i].Score == out[j].Score && out[i].LastSeen > out[j].LastSeen
	})
	return out[:min(len(out), 100)], nil
}

// Score compares player p with a player banned at banTS.
func Score(p, banned *player, banTS int64) Suspect {
	s := Suspect{Key: p.key, NameRaw: p.name, FirstSeen: p.first, LastSeen: p.last, ConnID: p.conn}
	add := func(code string, pts int, detail string) {
		s.Reasons = append(s.Reasons, Reason{code, pts, detail})
		s.Score += pts
	}
	if ip := common(p.ips, banned.ips); ip != "" {
		add("ip", ptIP, ip)
	} else if net := commonSubnet(p.ips, banned.ips); net != "" {
		add("subnet", ptSubnet, net+".x")
	}
	if id := common(p.steam, banned.steam); id != "" {
		add("steam", ptSteam, id)
	}
	sameProfile := p.level > 0 && banned.level > 0 && p.prestige == banned.prestige && banned.k >= 100 && p.k >= banned.k && p.k-banned.k <= 3000
	if sameProfile {
		add("profile", ptProfile, fmt.Sprintf("P%d · %d / %d kills", p.prestige, banned.k, p.k))
	}
	if p.settings != "" && p.settings == banned.settings && p.settings != "25000/20/30" {
		add("settings", ptSettings, p.settings)
	}
	if a, b := normName(p.clean), normName(banned.clean); a != "" && a == b {
		add("name", ptName, banned.clean)
	} else if len(a) >= 4 && len(b) >= 4 && levenshtein(a, b) <= 2 {
		add("name_near", ptNameNear, banned.clean)
	}
	// weak or generic clues: only on top of a clue linking the player to this banned player
	if s.Score == 0 {
		return s
	}
	// a player already known before the ban played alongside the banned one: only a strong clue makes them suspect
	if p.first <= banTS && !strong(s.Reasons) {
		return Suspect{Key: p.key}
	}
	if p.kills >= 20 && banned.kills >= 20 && p.dist.Valid && banned.dist.Valid &&
		math.Abs(pct(p.hs, p.kills)-pct(banned.hs, banned.kills)) <= 7 && math.Abs(pct(p.ns, p.kills)-pct(banned.ns, banned.kills)) <= 7 &&
		math.Abs(p.dist.Float64-banned.dist.Float64) <= 6 {
		add("style", ptStyle, fmt.Sprintf("HS %.0f %% · NS %.0f %% · %.0f m", pct(p.hs, p.kills), pct(p.ns, p.kills), p.dist.Float64))
	}
	if newGUID := p.first > banTS; newGUID {
		if !sameProfile && (p.level >= 10 || p.prestige > 0) {
			add("veteran", ptVeteran, fmt.Sprintf("P%d · lv %d · %d kills", p.prestige, p.level, p.k))
		}
		add("new", ptNew, "")
	}
	return s
}

// load gathers every known player: connections (IPs, Steam ids), latest profile and settings, kill style.
func load(db *sql.DB, server int64) (map[string]*player, error) {
	players := map[string]*player{}
	get := func(key string) *player {
		p := players[key]
		if p == nil {
			p = &player{key: key, ips: map[string]bool{}, steam: map[string]bool{}}
			players[key] = p
		}
		return p
	}
	rs, err := db.Query(`SELECT COALESCE(guid, 'n:'||name_clean), name_raw, name_clean, ip, COALESCE(steamid, ''), joined_at, id
		FROM connections WHERE server_id=? ORDER BY joined_at`, server)
	if err != nil {
		return nil, err
	}
	for rs.Next() {
		var key, name, clean, ip, steam string
		var at, id int64
		if err := rs.Scan(&key, &name, &clean, &ip, &steam, &at, &id); err != nil {
			rs.Close()
			return nil, err
		}
		p := get(key)
		if p.first == 0 {
			p.first = at
		}
		p.last, p.conn, p.name, p.clean = at, id, name, clean
		if ip != "" {
			p.ips[ip] = true
		}
		if steam != "" && steam != "0" {
			p.steam[steam] = true
		}
	}
	rs.Close()
	queries := []struct {
		sql  string
		scan func(*sql.Rows) error
	}{
		{`SELECT key, level, prestige, kills FROM player_profiles WHERE id IN (SELECT MAX(id) FROM player_profiles WHERE server_id=? GROUP BY key)`,
			func(r *sql.Rows) error {
				var key string
				var lv, pr, k int
				err := r.Scan(&key, &lv, &pr, &k)
				if p := players[key]; p != nil {
					p.level, p.prestige, p.k = lv, pr, k
				}
				return err
			}},
		{`SELECT key, rate||'/'||snaps||'/'||maxpackets FROM player_fingerprints WHERE id IN (SELECT MAX(id) FROM player_fingerprints WHERE server_id=? GROUP BY key)`,
			func(r *sql.Rows) error {
				var key, fp string
				err := r.Scan(&key, &fp)
				if p := players[key]; p != nil {
					p.settings = fp
				}
				return err
			}},
		{`SELECT killer_key, COUNT(*), SUM(cause='MOD_HEAD_SHOT'), SUM(','||COALESCE(tags, '')||',' LIKE '%,ns,%'), AVG(distance) FROM kills
			WHERE server_id=? AND killer_key IS NOT NULL AND teamkill=0 GROUP BY killer_key`,
			func(r *sql.Rows) error {
				var key string
				var k, hs, ns int
				var d sql.NullFloat64
				err := r.Scan(&key, &k, &hs, &ns, &d)
				if p := players[key]; p != nil {
					p.kills, p.hs, p.ns, p.dist = k, hs, ns, d
				}
				return err
			}},
	}
	for _, q := range queries {
		rs, err := db.Query(q.sql, server)
		if err != nil {
			return nil, err
		}
		for rs.Next() {
			if err := q.scan(rs); err != nil {
				rs.Close()
				return nil, err
			}
		}
		rs.Close()
	}
	return players, nil
}

func strong(rs []Reason) bool {
	for _, r := range rs {
		switch r.Code {
		case "ip", "steam", "profile", "name":
			return true
		}
	}
	return false
}

func common(a, b map[string]bool) string {
	for k := range a {
		if b[k] {
			return k
		}
	}
	return ""
}

func commonSubnet(a, b map[string]bool) string {
	nets := map[string]bool{}
	for ip := range b {
		if i := strings.LastIndexByte(ip, '.'); i > 0 {
			nets[ip[:i]] = true
		}
	}
	for ip := range a {
		if i := strings.LastIndexByte(ip, '.'); i > 0 && nets[ip[:i]] {
			return ip[:i]
		}
	}
	return ""
}

var leet = strings.NewReplacer("0", "o", "1", "i", "3", "e", "4", "a", "5", "s", "7", "t", "@", "a", "$", "s", "|", "i")

// normName: lower case, l33t letters back to letters, only letters and digits kept.
func normName(n string) string {
	n = leet.Replace(strings.ToLower(n))
	return strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, n)
}

func levenshtein(a, b string) int {
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur := make([]int, len(b)+1)
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev = cur
	}
	return prev[len(b)]
}

func pct(a, b int) float64 { return 100 * float64(a) / float64(max(b, 1)) }
