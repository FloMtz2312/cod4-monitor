package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestImports(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "i.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	id, _ := st.SaveServerConf(store.ServerConf{Name: "S", Host: "203.0.113.10", Port: 28960, Enabled: true})
	now := time.Now()
	h, _ := hashPassword("password123")
	for u, role := range map[string]string{"owner": "admin", "other": "admin"} {
		st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at,account_role) VALUES(?,?,?,?)`, u, h, now.Unix(), role)
	}
	st.DB.Exec(`UPDATE servers SET owner='owner' WHERE id=?`, id)
	im := NewImports(st, t.TempDir(), 1<<20, nil)
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123", Imports: im})
	req := func(method, path, body, user, origin, ctype string) *httptest.ResponseRecorder {
		q := httptest.NewRequest(method, path, strings.NewReader(body))
		q.Host = "monitor.example.com"
		if ctype != "" {
			q.Header.Set("Content-Type", ctype)
		}
		if origin != "" {
			q.Header.Set("Origin", origin)
		}
		token := strings.Repeat(user[:1], 64)
		st.DB.Exec(`INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)`, tokenHash(token), user, now.Unix(), now.Add(time.Hour).Unix())
		q.Header.Set("Cookie", sessionCookie+"="+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, q)
		return w
	}
	const site = "https://monitor.example.com"
	file := "Tue Sep  1 20:00:30 2026: ^7Client Alpha 1.2.3.4:28960 ^7entered this server in slot 0 with playerid 0\n"
	path := "/api/serverconf/1/imports?name=enterleave.log"
	for _, c := range []struct {
		user, origin, ctype string
		code                int
	}{
		{"other", site, "application/octet-stream", 403},                   // not the owner
		{"owner", "https://evil.example", "application/octet-stream", 403}, // another website
		{"owner", "", "application/octet-stream", 403},                     // no Origin
		{"owner", site, "application/x-www-form-urlencoded", 403},          // a form
		{"owner", site, "application/octet-stream", 200},
	} {
		if w := req("PUT", path, file, c.user, c.origin, c.ctype); w.Code != c.code {
			t.Fatalf("%+v: %d %s", c, w.Code, w.Body)
		}
	}
	if w := req("PUT", path, strings.Repeat("x", 2<<20), "owner", site, "application/octet-stream"); w.Code != 413 {
		t.Fatalf("too big: %d", w.Code)
	}

	j, found, _ := st.NextImportJob()
	if !found {
		t.Fatal("no job")
	}
	im.process(context.Background(), j)
	w := req("GET", "/api/serverconf/1/imports", "", "owner", "", "")
	var out struct{ Imports []store.ImportJob }
	json.Unmarshal(w.Body.Bytes(), &out)
	if len(out.Imports) != 1 || out.Imports[0].State != store.ImportDone || out.Imports[0].Kind != "enterleave" || out.Imports[0].Imported != 1 {
		t.Fatalf("jobs %s", w.Body)
	}
	if w := req("GET", "/api/serverconf/1/imports", "", "other", "", ""); w.Code != 403 {
		t.Fatalf("other owner sees the imports: %d", w.Code)
	}
}
