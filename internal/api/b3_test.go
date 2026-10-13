package api

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/store"
)

func TestB3Routes(t *testing.T) {
	dir := t.TempDir()
	defer func(old string) { b3.Root = old }(b3.Root)
	b3.SetRoot(dir)
	dbPath := filepath.Join(dir, "b3.db")
	schema, err := os.ReadFile("../b3/testdata/b3.sql")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := sql.Open("sqlite", dbPath)
	if _, err := raw.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	raw.Exec(`INSERT INTO clients (guid, name, group_bits, time_add, time_edit) VALUES ('222', 'Player', 1, 1, 1)`)
	raw.Close()

	st, err := store.Open(filepath.Join(dir, "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.SaveServerConf(store.ServerConf{Name: "S", Host: "127.0.0.1", Port: 28960, Enabled: true, Logs: store.LogFiles{B3DB: dbPath}})
	if err != nil {
		t.Fatal(err)
	}
	other, _ := st.SaveServerConf(store.ServerConf{Name: "No B3", Host: "127.0.0.1", Port: 28961, Enabled: true})
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("user", "florian") })
	b3Routes(r.Group("/api/servers/:id"), st, nil)
	do := func(method, path, body string) (int, map[string]any) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "localhost"
		r.ServeHTTP(w, req)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	base := "/api/servers/" + itoa(id) + "/b3"
	if code, _ := do("GET", "/api/servers/"+itoa(other)+"/b3", ""); code != 404 {
		t.Fatalf("server without B3: %d", code)
	}
	if code, out := do("POST", base+"/penalty", `{"guid":"222","name":"^1Player","type":"TempBan","minutes":0,"reason":"x"}`); code != 400 {
		t.Fatalf("temp ban without duration: %d %v", code, out)
	}
	if code, out := do("POST", base+"/penalty", `{"guid":"222","name":"^1Player","type":"TempBan","minutes":60,"reason":"wall hack"}`); code != 200 {
		t.Fatalf("temp ban: %d %v", code, out)
	}
	code, out := do("GET", base+"/penalties?active=1", "")
	ps, _ := out["penalties"].([]any)
	if code != 200 || len(ps) != 1 || ps[0].(map[string]any)["admin"] != "site: florian" {
		t.Fatalf("penalties: %d %v", code, out)
	}
	if code, out := do("POST", base+"/group", `{"guid":"222","name":"Player","group":8}`); code != 200 {
		t.Fatalf("group: %d %v", code, out)
	}
	code, out = do("GET", base+"/player?guid=222", "")
	cl, _ := out["client"].(map[string]any)
	if code != 200 || cl == nil || cl["group"].(map[string]any)["level"] != float64(20) {
		t.Fatalf("player: %d %v", code, out)
	}
	if code, out := do("POST", base+"/unban", `{"guid":"222","name":"Player"}`); code != 200 || out["lifted"] != float64(1) {
		t.Fatalf("unban: %d %v", code, out)
	}
	if code, out := do("GET", base+"/stats?days=30", ""); code != 200 || out["totals"].(map[string]any)["TempBan"] != float64(1) {
		t.Fatalf("stats: %d %v", code, out)
	}
	if code, out := do("GET", base+"/admins", ""); code != 200 || len(out["admins"].([]any)) != 1 {
		t.Fatalf("admins: %d %v", code, out)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
