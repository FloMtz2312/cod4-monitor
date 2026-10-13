package api

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/store"
)

func TestMatchesAndAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	base := time.Now().Add(-2 * time.Hour)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }
	kill := func(s int, killer, victim string) {
		if err := st.AddKill(1, at(s), killer, killer, killer, "axis", victim, victim, victim, "allies", "m40a3_mp", "MOD_HEAD_SHOT", "head", 100); err != nil {
			t.Fatal(err)
		}
	}
	// match 1 on mp_crash: two rounds; match 2 on mp_bog; match 3 on mp_bog again after a 20 min pause
	st.StartRound(1, at(0), "mp_crash", "sd")
	kill(10, "a", "b")
	kill(20, "a", "c")
	st.StartRound(1, at(300), "mp_crash", "sd")
	kill(310, "b", "a")
	st.StartRound(1, at(600), "mp_bog", "sd")
	kill(610, "c", "b")
	st.EndRound(1, at(900))
	st.StartRound(1, at(2100), "mp_bog", "sd")
	kill(2110, "c", "a")
	st.EndRound(1, at(2400))
	// connections: x stays the whole crash match, y leaves in the middle of it; a ban and its evasion
	db := st.DB
	conn := func(guid, ip string, a, b int) {
		db.Exec(`INSERT INTO connections(server_id, slot, addr, ip, guid, name_raw, name_clean, joined_at, left_at, open) VALUES(1,0,?,?,?,?,?,?,?,0)`,
			ip+":1", ip, guid, guid, guid, at(a).Unix(), at(b).Unix())
	}
	conn("x", "1.1.1.1", -60, 700)
	conn("y", "2.2.2.2", -60, 200)
	conn("banned", "3.3.3.3", -60, 100)
	st.AddBan(1, at(150), "banned", "Banned", "3.3.3.3", "", "cheat", "permban 1 cheat")
	conn("alt", "3.3.3.3", 1000, 1100)

	r := gin.New()
	r.Use(audit(db))
	g := r.Group("/api/servers/:id")
	matchRoutes(g, db)
	featureRoutes(g, st, nil)
	get := func(path string, out any) {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body)
		}
		json.Unmarshal(w.Body.Bytes(), out)
	}

	var ms struct {
		Matches []struct {
			Match    match
			Players  int
			TopKey   string `json:"top_key"`
			TopKills int    `json:"top_kills"`
		}
	}
	get("/api/servers/1/matches?range=24h", &ms)
	if len(ms.Matches) != 3 || ms.Matches[2].Match.Map != "mp_crash" || ms.Matches[2].Match.RoundCount != 2 ||
		ms.Matches[2].TopKey != "a" || ms.Matches[2].TopKills != 2 || ms.Matches[2].Players != 3 {
		t.Fatalf("matches: %+v", ms.Matches)
	}
	db.Exec(`UPDATE rounds SET winner='allies', score_allies=1, score_axis=1 WHERE map='mp_crash' AND started_at=?`, at(300).Unix())
	var mine struct{ Matches []map[string]any }
	get("/api/servers/1/matches?range=24h&key=a", &mine) // a: crash (2 kills, killed as allies) and the 2nd bog match
	if len(mine.Matches) != 2 || mine.Matches[1]["match"].(map[string]any)["map"] != "mp_crash" {
		t.Fatalf("player matches: %+v", mine.Matches)
	}
	if p := mine.Matches[1]["player"].(map[string]any); p["kills"] != 2.0 || p["deaths"] != 1.0 || p["team"] != "allies" || p["result"] != "win" {
		t.Fatalf("player in crash: %+v", p)
	}
	if p := mine.Matches[0]["player"].(map[string]any); p["kills"] != 0.0 || p["deaths"] != 1.0 || p["result"] != "" {
		t.Fatalf("player in bog: %+v", p)
	}
	var one struct {
		Players []map[string]any
		Rounds  []map[string]any
	}
	get("/api/servers/1/match?id="+jsonNum(ms.Matches[2].Match.ID+1), &one) // any round of the match
	if len(one.Rounds) != 2 || one.Players[0]["key"] != "a" || one.Players[0]["kills"] != 2.0 || one.Players[0]["deaths"] != 1.0 ||
		one.Players[0]["team"] != "allies" { // killed as allies in the last round
		t.Fatalf("match: %+v", one)
	}
	var mapst struct{ Maps []map[string]any }
	get("/api/servers/1/mapstats?range=24h", &mapst)
	if mapst.Maps[0]["map"] != "mp_bog" || mapst.Maps[0]["matches"] != 2.0 {
		t.Fatalf("mapstats: %+v", mapst.Maps)
	}
	for _, m := range mapst.Maps {
		if m["map"] == "mp_crash" && m["left_rate"] != 2.0/3 { // y and banned left during it, x stayed
			t.Fatalf("crash left rate: %+v", m)
		}
	}
	var ev struct{ Evasions []map[string]any }
	get("/api/servers/1/evasion", &ev)
	if len(ev.Evasions) != 1 || ev.Evasions[0]["key"] != "alt" || ev.Evasions[0]["ban_guid"] != "banned" {
		t.Fatalf("evasion: %+v", ev.Evasions)
	}
	var b struct{ Badges []map[string]any }
	get("/api/servers/1/badges?range=24h", &b)
	if len(b.Badges) == 0 || b.Badges[0]["badge"] != "killer" {
		t.Fatalf("badges: %+v", b.Badges)
	}
	// combat ranking with the mod's details (distance, tags, round winners, clutches)
	db.Exec(`UPDATE kills SET distance=55.5, tags='hs,ns' WHERE killer_key='a'`)
	db.Exec(`UPDATE rounds SET winner='axis'`)
	db.Exec(`INSERT INTO game_events(server_id, ts, round_id, type, key, name_raw, value) VALUES(1, ?, 1, 'clutch', 'a', 'a', 2)`, at(20).Unix())
	statsRoutes(g, db)
	var cb struct{ Players []map[string]any }
	get("/api/servers/1/combat?range=24h", &cb)
	var pa map[string]any
	for _, x := range cb.Players {
		if x["key"] == "a" {
			pa = x
		}
	}
	if pa == nil || pa["noscopes"] != 2.0 || pa["longest"] != 55.5 || pa["clutches"] != 1.0 || pa["rounds_played"] != 3.0 || pa["rounds_won"] != 1.0 {
		t.Fatalf("combat: %+v", cb.Players)
	}
	var pr struct {
		Sniper  []map[string]any
		NameRaw string `json:"name_raw"`
	}
	get("/api/servers/1/player?key=a", &pr)
	if len(pr.Sniper) != 1 || pr.Sniper[0]["noscopes"] != 2.0 || pr.Sniper[0]["clutches"] != 1.0 || pr.NameRaw != "a" {
		t.Fatalf("player sniper: %+v", pr.Sniper)
	}
	var rec struct{ Records map[string]map[string]any }
	get("/api/servers/1/records?range=24h", &rec)
	if rec.Records["longest"]["Value"] != 56.0 || rec.Records["longest_noscope"]["Key"] != "a" {
		t.Fatalf("records: %+v", rec.Records)
	}
	var mt struct{ Rounds []map[string]any }
	get("/api/servers/1/match?id="+jsonNum(ms.Matches[2].Match.ID), &mt)
	if mt.Rounds[0]["winner"] != "axis" || mt.Rounds[0]["first_blood"] != "a" || !strings.Contains(fmt.Sprint(mt.Rounds[0]["events"]), "clutch:a:2") {
		t.Fatalf("match rounds: %+v", mt.Rounds)
	}

	var p struct{ Weeks []map[string]any }
	get("/api/servers/1/progress?key=a", &p)
	if len(p.Weeks) == 0 || p.Weeks[len(p.Weeks)-1]["kills"] != 2.0 {
		t.Fatalf("progress: %+v", p.Weeks)
	}

	// audit: a successful change is logged, a refused one isn't
	put := func(body string) int {
		req := httptest.NewRequest("PUT", "/api/servers/1/notes", strings.NewReader(body))
		req.Host = "localhost:8080" // without a login, changes are only accepted from the machine itself
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	put(`{"key":"a","tag":"vip","note":"ok"}`)
	put(`{"key":""}`)
	var n int
	var who, action, detail string
	db.QueryRow(`SELECT COUNT(*), MAX(username), MAX(action), MAX(detail) FROM admin_log`).Scan(&n, &who, &action, &detail)
	if n != 1 || who != "local" || action != "Note joueur" || detail != "a vip" { // "a" has no GUID: its key is its name
		t.Fatalf("audit: %d %s %s %s", n, who, action, detail)
	}
}

func jsonNum(n int64) string { b, _ := json.Marshal(n); return string(b) }
