package api

import (
	"encoding/json"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	"path/filepath"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestRetention(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	now := time.Date(2026, 10, 7, 20, 0, 0, 0, time.Local)
	day := func(d int) int64 { return now.AddDate(0, 0, d).Unix() }
	visit := func(guid string, start int64, secs int64) {
		if _, err := st.DB.Exec(`INSERT INTO connections(server_id, slot, addr, ip, guid, name_raw, name_clean, joined_at, left_at, open)
			VALUES(1, 0, 'a', 'a', ?, ?, ?, ?, ?, 0)`, guid, guid, guid, start, start+secs); err != nil {
			t.Fatal(err)
		}
	}
	// regular who stopped coming: 5 days, last 20 days ago
	for d := -40; d <= -20; d += 5 {
		visit("reg", day(d), 3600)
	}
	visit("new1", day(-15), 1800) // new, came back 2 days later
	visit("new1", day(-13), 1800)
	visit("new2", day(-12), 1800) // new, never came back
	visit("new3", day(-1), 1800)  // new, too early to tell
	visit("blip", day(-3), 20)    // 20 s: doesn't count

	r, err := retention(st.DB, "1", day(-30), now.Unix(), now)
	if err != nil {
		t.Fatal(err)
	}
	if r["new"] != 3 || r["active"] != 4 || r["returning"] != 1 || r["measurable"] != 2 || r["returned"] != 1 {
		t.Fatalf("summary: new=%v active=%v returning=%v measurable=%v returned=%v", r["new"], r["active"], r["returning"], r["measurable"], r["returned"])
	}
	if g := r["gone"].([]gin.H); len(g) != 1 || g[0]["key"] != "reg" || g[0]["days"] != 5 {
		t.Fatalf("gone: %v", g)
	}
}

func TestMapRoundsAndPings(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	at := func(s int64) time.Time { return time.Unix(1_000_000+s, 0) }
	st.StartRound(1, at(0), "mp_crash", "sd")
	st.AddKill(1, at(10), "a", "a", "a", "axis", "b", "b", "b", "allies", "m40a3_mp", "MOD_RIFLE_BULLET", "torso_upper", 100)
	st.FinishRound(1, at(60), "axis", 0, 1, "axis")    // attack wins, 60 s
	st.FinishRound(1, at(180), "allies", 1, 1, "axis") // defense wins, 120 s
	st.FinishRound(1, at(270), "tie", 1, 1, "axis")    // neither
	st.EndRound(1, at(280))

	r := httpGet(t, st, "/api/servers/1/maprounds?from=999000&to=1001000")
	m := r["maps"].([]any)[0].(map[string]any)
	if m["rounds"] != 3.0 || m["attack"] != 1.0 || m["defense"] != 1.0 || m["avg_s"] != 90.0 || m["matches"] != 1.0 {
		t.Fatalf("maprounds: %v", m)
	}

	for i, ping := range []int{50, 70, 999, 200, 60} {
		res, _ := st.DB.Exec(`INSERT INTO snapshots(server_id, ts, online) VALUES(1, ?, 1)`, 1_000_000+int64(i)*15)
		id, _ := res.LastInsertId()
		st.DB.Exec(`INSERT INTO player_sightings VALUES(?, '^1Bob', 'Bob', 0, ?)`, id, ping)
	}
	p := httpGet(t, st, "/api/servers/1/pings?from=999000&to=1001000")
	pl := p["players"].([]any)[0].(map[string]any)
	if pl["samples"] != 4.0 || pl["avg"] != 95.0 || pl["max"] != 200.0 || pl["high"] != 0.25 || p["avg"] != 95.0 {
		t.Fatalf("pings: %v", p)
	}
}

func httpGet(t *testing.T, st *store.Store, path string) map[string]any {
	t.Helper()
	r := gin.New()
	insightRoutes(r.Group("/api/servers/:id"), st.DB)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	if w.Code != 200 {
		t.Fatalf("%s: %d %s", path, w.Code, w.Body)
	}
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

func TestLiveMatch(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	at := func(s int64) time.Time { return time.Unix(1_000_000+s, 0) }
	st.StartRound(1, at(0), "mp_backlot", "sd") // an older match on another map
	st.FinishRound(1, at(100), "axis", 0, 1, "axis")
	st.EndRound(1, at(110))
	st.StartRound(1, at(200), "mp_crash", "sd")
	st.FinishRound(1, at(300), "axis", 0, 1, "axis")
	st.FinishRound(1, at(400), "allies", 1, 1, "allies")
	m, err := liveMatch(st.DB, "1", 1_000_000+450)
	if err != nil {
		t.Fatal(err)
	}
	if m["map"] != "mp_crash" || m["rounds"] != 2 || m["score_allies"] != int64(1) || m["score_axis"] != int64(1) || m["started_at"] != int64(1_000_200) {
		t.Fatalf("live: %v", m)
	}
	// FinishRound opened the next round: a long round (no time limit) keeps the match live
	if m, _ := liveMatch(st.DB, "1", 1_000_000+400+matchGap+60); m == nil || m["rounds"] != 2 || m["score_allies"] != int64(1) {
		t.Fatalf("open round: %v", m)
	}
	st.EndRound(1, at(420)) // server stopped: the empty round is dropped
	if m, _ := liveMatch(st.DB, "1", 1_000_000+400+matchGap+60); m != nil {
		t.Fatalf("old match still live: %v", m)
	}
	st.StartRound(1, at(500), "mp_crash", "sd") // never ended (crash)
	if m, _ := liveMatch(st.DB, "1", 1_000_000+500+openRound+60); m != nil {
		t.Fatalf("stale open round still live: %v", m)
	}
}
