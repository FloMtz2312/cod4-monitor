package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

// The notes permission alone sees and edits the admin note of a player, not the rest of the private data.
func TestNotesPermission(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "notes.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	st.SaveServerConf(store.ServerConf{Name: "One", Host: "one.example", Port: 28960, Enabled: true})
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123"})
	h, _ := hashPassword("password123")
	now := time.Now().Unix()
	st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at,account_role) VALUES('noter',?,?,'moderator'),('kicker',?,?,'moderator')`, h, now, h, now)
	st.DB.Exec(`INSERT INTO server_members(server_id,username,permissions) VALUES(1,'noter','["notes"]'),(1,'kicker','["kick"]')`)
	st.DB.Exec(`INSERT INTO player_notes VALUES(1, 'g1', 'suspect', 'aimbot ?', 0)`)
	st.SeenName(1, time.Now(), "g1", "Ace", "Ace")
	request := func(method, path, body, user string) *httptest.ResponseRecorder {
		q := httptest.NewRequest(method, path, strings.NewReader(body))
		q.Header.Set("Content-Type", "application/json")
		token := strings.Repeat(user[:1], 64)
		st.DB.Exec(`INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)`, tokenHash(token), user, now, now+3600)
		q.Header.Set("Cookie", sessionCookie+"="+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		return w
	}
	profile := func(user string) map[string]any {
		w := request("GET", "/api/servers/1/player?key=g1", "", user)
		var out map[string]any
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
			t.Fatalf("%s: %d %s", user, w.Code, w.Body)
		}
		return out
	}
	if p := profile("noter"); p["note"] == nil || p["ips"] != nil || p["chat"] != nil {
		t.Fatalf("notes permission: note %v, ips %v, chat %v", p["note"], p["ips"], p["chat"])
	}
	if p := profile("kicker"); p["note"] != nil {
		t.Fatal("note shown without the notes permission")
	}
	if w := request("GET", "/api/servers/1/notes", "", "noter"); w.Code != 200 {
		t.Fatalf("notes list: %d", w.Code)
	}
	if w := request("GET", "/api/servers/1/notes", "", "kicker"); w.Code != 403 {
		t.Fatalf("notes list without the permission: %d", w.Code)
	}
}
