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

// A server owner connects B3 kept in MySQL: the password is encrypted, never sent back, kept when the form leaves it
// empty; an address of the monitor's own network is only allowed to the super administrator.
func TestB3MySQLSettings(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "b3m.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123"})
	h, _ := hashPassword("password123")
	now := time.Now().Unix()
	st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at,account_role) VALUES('alice',?,?,'admin')`, h, now)
	request := func(method, path, body, user string) (int, map[string]any) {
		q := httptest.NewRequest(method, path, strings.NewReader(body))
		q.Header.Set("Content-Type", "application/json")
		token := strings.Repeat(user[:1], 64)
		st.DB.Exec(`INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)`, tokenHash(token), user, now, now+3600)
		q.Header.Set("Cookie", sessionCookie+"="+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	form := func(host, pass string) string {
		return `{"name":"Alice","address":"203.0.113.9:28960","b3_mysql":{"host":"` + host + `","port":3307,"user":"b3","pass":"` + pass + `","name":"b3db","tls":true}}`
	}

	if code, out := request("POST", "/api/serverconf", form("10.0.0.5", "s3cret"), "alice"); code != 400 || !strings.Contains(out["error"].(string), "privée") {
		t.Fatalf("private address accepted for an owner: %d %v", code, out)
	}
	if code, out := request("POST", "/api/serverconf", `{"name":"A","address":"203.0.113.9:28960","b3_db":"/b3/b3.db"}`, "alice"); code != 403 {
		t.Fatalf("SQLite file accepted for an owner: %d %v", code, out)
	}
	code, out := request("POST", "/api/serverconf", form("203.0.113.20", "s3cret"), "alice")
	if code != 200 {
		t.Fatalf("create: %d %v", code, out)
	}
	id := int64(out["id"].(float64))
	c, _, _ := st.ServerConf(id)
	if c.Logs.B3DB != "mysql://b3:s3cret@203.0.113.20:3307/b3db?tls=true" {
		t.Fatalf("saved %q", c.Logs.B3DB)
	}
	var raw string
	st.DB.QueryRow(`SELECT b3_db FROM servers WHERE id=?`, id).Scan(&raw)
	if strings.Contains(raw, "s3cret") {
		t.Fatal("MySQL password stored in clear")
	}
	_, list := request("GET", "/api/serverconf", "", "alice")
	b, _ := json.Marshal(list)
	if strings.Contains(string(b), "s3cret") || !strings.Contains(string(b), `"has_pass":true`) || !strings.Contains(string(b), `"host":"203.0.113.20"`) {
		t.Fatalf("settings view: %s", b)
	}
	// an edit without the password keeps it; clearing removes the connection
	if code, out := request("PUT", "/api/serverconf/"+itoa(id), form("203.0.113.21", ""), "alice"); code != 200 {
		t.Fatalf("edit: %d %v", code, out)
	}
	if c, _, _ = st.ServerConf(id); c.Logs.B3DB != "mysql://b3:s3cret@203.0.113.21:3307/b3db?tls=true" {
		t.Fatalf("edited %q", c.Logs.B3DB)
	}
	if code, out := request("PUT", "/api/serverconf/"+itoa(id), `{"name":"Alice","address":"203.0.113.9:28960","b3_clear":true}`, "alice"); code != 200 {
		t.Fatalf("clear: %d %v", code, out)
	}
	if c, _, _ = st.ServerConf(id); c.Logs.B3DB != "" {
		t.Fatalf("not cleared: %q", c.Logs.B3DB)
	}
	// the super administrator may use a local MySQL (B3 on the same machine)
	if code, out := request("PUT", "/api/serverconf/"+itoa(id), form("127.0.0.1", "x"), "root"); code != 200 {
		t.Fatalf("super admin local MySQL: %d %v", code, out)
	}
}
