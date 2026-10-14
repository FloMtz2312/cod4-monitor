package api

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestMonitorLogs(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "l.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	now := time.Now()
	h, _ := hashPassword("password123")
	for u, role := range map[string]string{"boss": "superadmin", "owner": "admin"} {
		st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at,account_role) VALUES(?,?,?,?)`, u, h, now.Unix(), role)
	}
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123",
		MonitorLogs: func() []string { return []string{`{"msg":"log tail connected"}`} }})
	get := func(user string) *httptest.ResponseRecorder {
		q := httptest.NewRequest("GET", "/api/monitor-logs", nil)
		if user != "" {
			token := strings.Repeat(user[:1], 64)
			st.DB.Exec(`INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)`, tokenHash(token), user, now.Unix(), now.Add(time.Hour).Unix())
			q.Header.Set("Cookie", sessionCookie+"="+token)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		return w
	}
	if w := get(""); w.Code != 401 {
		t.Fatalf("anonymous: %d", w.Code)
	}
	if w := get("owner"); w.Code != 403 {
		t.Fatalf("server owner: %d", w.Code)
	}
	if w := get("boss"); w.Code != 200 || !strings.Contains(w.Body.String(), "log tail connected") {
		t.Fatalf("super admin: %d %s", w.Code, w.Body)
	}

	// without a dashboard password: local addresses only
	open := Router(Config{Store: st, Hub: NewHub(), MonitorLogs: func() []string { return nil }})
	for host, code := range map[string]int{"127.0.0.1:8080": 200, "monitor.example.com": 403} {
		q := httptest.NewRequest("GET", "/api/monitor-logs", nil)
		q.Host = host
		w := httptest.NewRecorder()
		open.ServeHTTP(w, q)
		if w.Code != code {
			t.Fatalf("no password, host %s: %d", host, w.Code)
		}
	}
}
