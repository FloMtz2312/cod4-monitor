package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/store"
)

func TestFeed(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	now := time.Now().Add(-5 * time.Minute)
	at := func(s int) time.Time { return now.Add(time.Duration(s) * time.Second) }
	// an older match on another map: not in the feed
	st.StartRound(1, now.Add(-2*time.Hour), "mp_crash", "sd")
	st.AddKill(1, now.Add(-2*time.Hour+time.Minute), "o", "^1Old", "Old", "axis", "p", "Prev", "Prev", "allies", "m40a3_mp", "MOD_RIFLE_BULLET", "torso_upper", 100)
	st.EndRound(1, now.Add(-2*time.Hour+2*time.Minute))
	// the running match: Babou (allies) kills Nova twice, Nova kills Babou once, then they switch sides
	st.StartRound(1, at(0), "mp_backlot", "sd")
	st.AddKill(1, at(10), "g1", "^2Babou", "Babou", "allies", "g2", "Nova", "Nova", "axis", "m40a3_mp", "MOD_HEAD_SHOT", "head", 100)
	st.AddKill(1, at(20), "g2", "Nova", "Nova", "axis", "g1", "^2Babou", "Babou", "allies", "remington700_mp", "MOD_RIFLE_BULLET", "torso_upper", 100)
	st.FinishRound(1, at(30), "allies", 1, 0, "axis")
	st.StartRound(1, at(40), "mp_backlot", "sd")
	st.AddKill(1, at(50), "g1", "^2Babou", "Babou", "axis", "g2", "Nova", "Nova", "allies", "m40a3_mp", "MOD_RIFLE_BULLET", "torso_upper", 100)

	r := gin.New()
	liveRoutes(r.Group("/api/servers/:id"), st.DB)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/servers/1/feed", nil))
	if w.Code != 200 {
		t.Fatalf("feed: %d %s", w.Code, w.Body)
	}
	var out struct {
		Match   map[string]any            `json:"match"`
		Events  []map[string]any          `json:"events"`
		Players map[string]map[string]any `json:"players"`
	}
	json.Unmarshal(w.Body.Bytes(), &out)
	if out.Match["map"] != "mp_backlot" || out.Match["score_allies"] != 1.0 {
		t.Fatalf("match: %v", out.Match)
	}
	types := ""
	for _, e := range out.Events {
		types += e["type"].(string) + " "
	}
	if types != "kill round kill kill " {
		t.Fatalf("events, newest first: %q", types)
	}
	if out.Events[0]["actor"] != "^2Babou" || out.Events[0]["target"] != "Nova" {
		t.Fatalf("last kill: %v", out.Events[0])
	}
	b, n := out.Players["^2Babou"], out.Players["Nova"]
	if b["team"] != "axis" || b["kills"] != 2.0 || b["deaths"] != 1.0 || b["key"] != "g1" {
		t.Fatalf("Babou (switched to axis): %v", b)
	}
	if n["team"] != "allies" || n["kills"] != 1.0 || n["deaths"] != 2.0 {
		t.Fatalf("Nova: %v", n)
	}
	if _, old := out.Players["^1Old"]; old {
		t.Fatal("a player of an older match is in the feed")
	}
}

func TestClaimMe(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	code, err := st.MeCode(1, "g1", "^2Babou", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	meRoutes(r, st)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/me/"+code, nil))
	if want := "/#/moi/" + code; w.Code != 302 || w.Header().Get("Location") != want {
		t.Fatalf("redirect: %d %q", w.Code, w.Header().Get("Location"))
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/claim/"+code, nil))
	var out map[string]any
	json.Unmarshal(w.Body.Bytes(), &out)
	if w.Code != 200 || out["key"] != "g1" || out["server_id"] != float64(1) || out["name_raw"] != "^2Babou" {
		t.Fatalf("claim: %d %v", w.Code, out)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/claim/ZZZZZ", nil))
	if w.Code != 404 {
		t.Fatalf("unknown code: %d", w.Code)
	}
}
