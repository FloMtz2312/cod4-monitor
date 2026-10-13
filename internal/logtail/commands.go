package logtail

import (
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Chat commands answered in game with rcon "tell" (needs the rcon password in SERVERS):
//   !stats [name]  kills, deaths, K/D, headshots, noscopes, longest kill (all time)
//   !top           top 5 killers of the last 7 days
//   !report <name> <reason>  stored for the admins (Admin tab), with a live alert on the dashboard
//   !me            a short link (or code) to open and pin the sender's profile on the dashboard
//   !help          (left to B3 when the server has one: its !help lists these commands)
// Only live lines are answered (not the lines caught up after a monitor restart).

const (
	cmdCooldown    = 3 * time.Second
	reportCooldown = time.Minute
	welcomeDelay   = 30 * time.Second // J is logged on connect, while the player is still loading
)

// safe keeps text from breaking out of the rcon command: ";" separates commands, quotes and line breaks end it.
func safe(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == ';' || r == '"' || r == '\\' || r < 0x20:
			return -1
		}
		return r
	}, s)
}

func (t *Tail) tell(slot int, msg string) {
	if t.Rcon == nil {
		return
	}
	if _, err := t.Rcon(fmt.Sprintf(`tell %d "%s"`, slot, safe(msg))); err != nil {
		slog.Warn("rcon tell", "server_id", t.ServerID, "err", err)
	}
}

// command handles a chat line starting with "!". It returns false for an unknown command.
func (t *Tail) command(st *store.Store, c Chat, emit func(map[string]any)) bool {
	name, arg, _ := strings.Cut(strings.TrimSpace(c.Message), " ")
	name, arg = strings.ToLower(name), strings.TrimSpace(arg)
	switch name {
	case "!stats", "!top", "!report", "!me":
	case "!help":
		if t.Files.B3DB != "" { // B3 runs on this server: its !help already lists !stats, !top, !report and !me
			return false
		}
	default:
		return false
	}
	t.mu.Lock()
	if t.lastCmd == nil {
		t.lastCmd = map[string]time.Time{}
	}
	key := fmt.Sprint(c.Slot)
	busy := time.Since(t.lastCmd[key]) < cmdCooldown || name == "!report" && time.Since(t.lastCmd["r"+key]) < reportCooldown
	if !busy {
		t.lastCmd[key] = time.Now()
		if name == "!report" {
			t.lastCmd["r"+key] = time.Now()
		}
	}
	t.mu.Unlock()
	if busy {
		return true
	}
	switch name {
	case "!help":
		t.tell(c.Slot, "^3!stats^7 [name] your stats  ^3!top^7 best of the week  ^3!report^7 name reason  ^3!me^7 your stats online")
	case "!stats":
		t.tell(c.Slot, statsLine(st.DB, t.ServerID, c, arg))
	case "!top":
		t.tell(c.Slot, topLine(st.DB, t.ServerID))
	case "!me":
		code, err := st.MeCode(t.ServerID, store.PlayerKey(c.GUID, c.NameClean), c.NameRaw, time.Now())
		if err != nil {
			slog.Error("store me code", "err", err)
			return true
		}
		t.tell(c.Slot, meLine(t.SiteURL, code))
	case "!report":
		target, reason, _ := strings.Cut(arg, " ")
		if target == "" {
			t.tell(c.Slot, "Usage: ^3!report^7 name reason")
			return true
		}
		if err := st.AddReport(t.ServerID, time.Now(), c.GUID, c.NameRaw, c.NameClean, target, strings.TrimSpace(reason)); err != nil {
			slog.Error("store report", "err", err)
			return true
		}
		emit(map[string]any{"type": "report", "reporter": c.NameRaw, "target": target, "reason": strings.TrimSpace(reason)})
		t.tell(c.Slot, "^2Report sent to the admins, thanks.")
	}
	return true
}

// meLine: the address to type on a phone (short, without https://), or the code to enter in the app.
func meLine(siteURL, code string) string {
	if host := strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(siteURL, "https://"), "http://"), "/"); host != "" {
		return "^7Your stats: ^3" + host + "/me/" + code + " ^7(valid 24 h)"
	}
	return "^7Your code: ^3" + code + " ^7- dashboard, Players tab, enter the code (valid 24 h)"
}

// playerKey: the sender, or the player whose (latest) name contains arg.
func playerKey(db *sql.DB, server int64, c Chat, arg string) (key, name string) {
	if arg == "" {
		return store.PlayerKey(c.GUID, c.NameClean), c.NameClean
	}
	var guid string
	if db.QueryRow(`SELECT guid, name_clean FROM player_names WHERE server_id=? AND name_clean LIKE '%'||?||'%' ORDER BY last_seen DESC LIMIT 1`,
		server, arg).Scan(&guid, &name) == nil {
		return guid, name
	}
	return "n:" + arg, arg
}

func statsLine(db *sql.DB, server int64, c Chat, arg string) string {
	key, name := playerKey(db, server, c, arg)
	var kills, deaths, hs, ns int
	var longest sql.NullFloat64
	err := db.QueryRow(`SELECT COALESCE(SUM(killer_key=?1 AND teamkill=0), 0), COALESCE(SUM(victim_key=?1), 0),
		COALESCE(SUM(killer_key=?1 AND cause='MOD_HEAD_SHOT'), 0), COALESCE(SUM(killer_key=?1 AND ','||tags||',' LIKE '%,ns,%'), 0),
		MAX(CASE WHEN killer_key=?1 AND teamkill=0 THEN distance END)
		FROM kills WHERE server_id=?2 AND (killer_key=?1 OR victim_key=?1)`, key, server).Scan(&kills, &deaths, &hs, &ns, &longest)
	if err != nil || kills+deaths == 0 {
		return "No stats yet for ^3" + name
	}
	s := fmt.Sprintf("^3%s^7: %d kills, %d deaths, K/D %.2f, HS %d%%, %d noscopes", name, kills, deaths,
		float64(kills)/float64(max(deaths, 1)), 100*hs/max(kills, 1), ns)
	if longest.Valid {
		s += fmt.Sprintf(", longest %.0fm", longest.Float64)
	}
	return s
}

func topLine(db *sql.DB, server int64) string {
	rs, err := db.Query(`SELECT (SELECT killer_name FROM kills k2 WHERE k2.server_id=?1 AND k2.killer_key=k.killer_key ORDER BY ts DESC LIMIT 1),
		COUNT(*) AS n FROM kills k WHERE server_id=?1 AND ts>=?2 AND killer_key IS NOT NULL AND teamkill=0
		GROUP BY killer_key ORDER BY n DESC LIMIT 5`, server, time.Now().AddDate(0, 0, -7).Unix())
	if err != nil {
		return "Top unavailable"
	}
	defer rs.Close()
	var parts []string
	for rs.Next() {
		var name string
		var n int
		rs.Scan(&name, &n)
		name = query.StripColors(name)
		if len(name) > 16 {
			name = name[:16]
		}
		parts = append(parts, fmt.Sprintf("%d. %s %d", len(parts)+1, name, n))
	}
	if len(parts) == 0 {
		return "No kills this week yet"
	}
	return "^3Top 7 days (kills)^7: " + strings.Join(parts, " | ")
}

// fingerprint keeps the network settings of a player who just connected (rcon dumpuser), for the ban evasion check.
func (t *Tail) fingerprint(st *store.Store, guid string, slot int, nameRaw string, emit func(map[string]any)) {
	if t.Rcon == nil || guid == "" || guid == "0" {
		return
	}
	time.Sleep(welcomeDelay)
	out, err := t.Rcon(fmt.Sprintf("dumpuser %d", slot))
	if err != nil {
		return
	}
	info := ParseUserinfo(out)
	if info["rate"] == "" || query.StripColors(info["name"]) != query.StripColors(nameRaw) { // unknown command, or another player in the slot
		return
	}
	if err := st.AddFingerprint(t.ServerID, time.Now(), guid, query.StripColors(nameRaw), info); err != nil {
		slog.Error("store fingerprint", "err", err)
		return
	}
	emit(map[string]any{"type": "profile"})
}

// welcome sends the admin's message to a player who just connected, once loaded, if the slot is still theirs.
func (t *Tail) welcome(st *store.Store, guid string, slot int, nameRaw string) {
	msg := st.Welcome(t.ServerID, guid, query.StripColors(nameRaw))
	if msg == "" || t.Rcon == nil {
		return
	}
	time.Sleep(welcomeDelay)
	out, err := t.Rcon("status")
	if err != nil {
		return
	}
	for _, p := range query.ParseRconStatus(out) {
		if p.Num == slot && query.StripColors(p.Name) == query.StripColors(nameRaw) {
			t.tell(slot, strings.ReplaceAll(msg, "{name}", query.StripColors(nameRaw)))
			return
		}
	}
}
