package api

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

func TestInbox(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	now := time.Now()
	at := func(s int) time.Time { return now.Add(time.Duration(s-3600) * time.Second) }
	st.SeenName(1, at(0), "cheat", "^1Cheat", "Cheat")
	st.SeenName(1, at(0), "norm", "Norm", "Norm")
	// Cheat: 24 headshots out of 25 kills; Norm: 20 kills, 4 headshots, killed 10 times by Cheat
	for i := range 25 {
		cause := "MOD_HEAD_SHOT"
		if i == 0 {
			cause = "MOD_RIFLE_BULLET"
		}
		st.AddKill(1, at(i), "cheat", "^1Cheat", "Cheat", "axis", "norm", "Norm", "Norm", "allies", "m40a3_mp", cause, "head", 100)
	}
	for i := range 20 {
		cause := "MOD_RIFLE_BULLET"
		if i < 4 {
			cause = "MOD_HEAD_SHOT"
		}
		st.AddKill(1, at(100+i), "norm", "Norm", "Norm", "allies", "x", "X", "X", "axis", "m40a3_mp", cause, "torso_upper", 100)
	}
	st.AddReport(1, at(200), "norm", "Norm", "Norm", "chea", "wallhack")
	// Norm is tagged "to watch" and plays now
	st.DB.Exec(`INSERT INTO player_notes VALUES(1, 'norm', 'surveiller', 'insults', 0)`)
	st.Record(1, now, &query.Status{Vars: map[string]string{"mapname": "mp_crash"}, Players: []query.Player{{NameRaw: "Norm", NameClean: "Norm"}}})

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("role", c.GetHeader("X-Role")); c.Set("user", "babou") })
	inboxRoutes(r.Group("/api/servers/:id"), st)
	list := func(role string) []map[string]any {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/servers/1/inbox", nil)
		req.Header.Set("X-Role", role)
		r.ServeHTTP(w, req)
		var out struct{ Items []map[string]any }
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatalf("inbox: %d %s", w.Code, w.Body)
		}
		return out.Items
	}
	types := func(items []map[string]any) map[string]map[string]any {
		m := map[string]map[string]any{}
		for _, it := range items {
			m[it["type"].(string)] = it
		}
		return m
	}

	got := types(list(roleAdmin))
	if rep := got["report"]; rep == nil || rep["key"] != "cheat" || rep["reason"] != "wallhack" {
		t.Fatalf("report: %v", got)
	}
	if s := got["suspect"]; s == nil || s["key"] != "cheat" || s["headshots"] != 24.0 {
		t.Fatalf("suspect (Norm's 20 %% must not be): %v", got)
	}
	if w := got["watch"]; w == nil || w["key"] != "norm" || w["tag"] != "surveiller" {
		t.Fatalf("watch: %v", got)
	}
	// reports only: no stats nor notes
	if only := types(list(roleVisitor)); len(only) != 1 || only["report"] == nil {
		t.Fatalf("reports-only view: %v", only)
	}

	// classify the report and the suspect: gone for everybody
	for _, id := range []string{got["report"]["id"].(string), got["suspect"]["id"].(string)} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/api/servers/1/inbox/dismiss", bytes.NewBufferString(`{"id":"`+id+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "localhost" // sameOrigin without login: local requests only
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("dismiss %s: %d %s", id, w.Code, w.Body)
		}
	}
	after := types(list(roleAdmin))
	if after["report"] != nil || after["suspect"] != nil || after["watch"] == nil {
		t.Fatalf("after dismiss: %v", after)
	}
	var by string
	st.DB.QueryRow(`SELECT handled_by FROM reports`).Scan(&by)
	if by != "babou" {
		t.Fatalf("report handled by %q", by)
	}
}
