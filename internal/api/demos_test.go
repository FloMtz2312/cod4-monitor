package api

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/logtail"
	"cod4monitor/internal/store"
)

func TestDemos(t *testing.T) {
	root := t.TempDir()
	logtail.SetLocalRoot(root)
	t.Cleanup(func() { logtail.LocalRoot = "/gamelogs/" })
	os.MkdirAll(filepath.Join(root, "mods", "m"), 0o755)
	os.MkdirAll(filepath.Join(root, "demos"), 0o755)
	os.WriteFile(filepath.Join(root, "demos", "demo_123_0000.dm_1"), []byte("DEMO-BYTES"), 0o644)

	st, err := store.Open(filepath.Join(t.TempDir(), "d.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	id, _ := st.SaveServerConf(store.ServerConf{Name: "S", Host: "203.0.113.10", Port: 28960, Enabled: true,
		LogURL: "file://" + root + "/mods/m/", RconPass: "pw"})
	now := time.Now()
	if err := st.SeeDemo(id, "demo_123_0000.dm_1", 10, now.Add(-time.Hour), now); err != nil {
		t.Fatal(err)
	}
	h, _ := hashPassword("password123")
	for _, u := range []string{"owner", "mod", "nobody"} {
		st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at) VALUES(?,?,?)`, u, h, now.Unix())
	}
	st.DB.Exec(`UPDATE servers SET owner='owner' WHERE id=?`, id)
	st.DB.Exec(`UPDATE users SET account_role='admin' WHERE username='owner'`)
	st.DB.Exec(`INSERT INTO server_members VALUES(?,?,?)`, id, "mod", `["private"]`)
	st.DB.Exec(`INSERT INTO server_members VALUES(?,?,?)`, id, "nobody", `["kick"]`)

	var sent []string
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123",
		Rcon: func(int64) func(string) (string, error) {
			return func(c string) (string, error) { sent = append(sent, c); return "", nil }
		}})
	req := func(method, path, body, user string) *httptest.ResponseRecorder {
		q := httptest.NewRequest(method, path, strings.NewReader(body))
		q.Header.Set("Content-Type", "application/json")
		if user != "" {
			token := strings.Repeat(user[:1], 64)
			st.DB.Exec(`INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)`, tokenHash(token), user, now.Unix(), now.Add(time.Hour).Unix())
			q.Header.Set("Cookie", sessionCookie+"="+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		return w
	}
	expect := func(w *httptest.ResponseRecorder, code int) {
		t.Helper()
		if w.Code != code {
			t.Fatalf("want %d got %d: %s", code, w.Code, w.Body)
		}
	}
	sid := "/api/servers/1"

	// list and file: private data
	expect(req("GET", sid+"/demos", "", ""), 401)
	expect(req("GET", sid+"/demos", "", "nobody"), 403)
	w := req("GET", sid+"/demos", "", "mod")
	expect(w, 200)
	var list struct{ Demos []store.Demo }
	json.Unmarshal(w.Body.Bytes(), &list)
	if len(list.Demos) != 1 || list.Demos[0].Player != "123" {
		t.Fatalf("demos %s", w.Body)
	}
	w = req("GET", sid+"/demos/1/file/demo_123_0000.dm_1", "", "mod")
	expect(w, 200)
	if w.Body.String() != "DEMO-BYTES" {
		t.Fatalf("file %q", w.Body)
	}
	expect(req("GET", sid+"/demos/1/file/x", "", "nobody"), 403)
	expect(req("POST", sid+"/demos/1/keep", `{"keep":true}`, "mod"), 200)
	if d, _, _ := st.Demo(id, 1); d.Kept != store.DemoManual {
		t.Fatalf("keep %+v", d)
	}

	// settings: the owner only
	conf := "/api/serverconf/1/demos"
	expect(req("GET", conf, "", "mod"), 403)
	expect(req("GET", conf, "", "owner"), 200)
	expect(req("PUT", conf, `{"enabled":true,"min_players":0,"keep_hours":48}`, "owner"), 400)
	expect(req("PUT", conf, `{"enabled":true,"min_players":2,"keep_hours":48,"dir":"/etc/"}`, "owner"), 400)
	expect(req("PUT", conf, `{"enabled":true,"min_players":2,"keep_hours":72}`, "owner"), 200)
	if s, _ := st.DemoSettings(id); !s.Enabled || s.KeepHours != 72 {
		t.Fatalf("settings %+v", s)
	}
	expect(req("GET", conf+"/estimate?min=2&keep=24", "", "owner"), 200)
	// turning it off stops the recording on the server at once
	sent = nil
	expect(req("PUT", conf, `{"enabled":false,"min_players":2,"keep_hours":72}`, "owner"), 200)
	if strings.Join(sent, "|") != "stoprecord all|set sv_autodemorecord 0" {
		t.Fatalf("disable sent %q", sent)
	}
}
