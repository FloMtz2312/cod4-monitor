package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/store"
	"github.com/gin-gonic/gin"
)

func TestTenantInvitationsAndIsolation(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "tenant.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	st.SaveServerConf(store.ServerConf{Name: "Legacy", Host: "legacy.example", Port: 28960, Enabled: true})
	h, _ := hashPassword("password123")
	st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at) VALUES('legacy',?,?)`, h, time.Now().Unix())
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123"})
	request := func(method, path, body, user string) *httptest.ResponseRecorder {
		q := httptest.NewRequest(method, path, strings.NewReader(body))
		q.Header.Set("Content-Type", "application/json")
		if user != "" {
			token := strings.Repeat(user[:1], 64)
			st.DB.Exec(`INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)`, tokenHash(token), user, time.Now().Unix(), time.Now().Add(time.Hour).Unix())
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
	invite := func(user, acct, creator string, id int64, ps []string) string {
		t.Helper()
		body, _ := json.Marshal(map[string]any{"username": user, "role": acct, "server_id": id, "permissions": ps})
		w := request("POST", "/api/invitations", string(body), creator)
		expect(w, 201)
		var out struct{ Token string }
		json.Unmarshal(w.Body.Bytes(), &out)
		return out.Token
	}
	accept := func(token string) *httptest.ResponseRecorder {
		return request("POST", "/api/invitations/accept", `{"token":"`+token+`","password":"password123"}`, "")
	}
	ta := invite("alice", "admin", "root", 0, nil)
	expect(accept(ta), 200)
	expect(accept(ta), 400)
	tb := invite("bob", "admin", "root", 0, nil)
	expect(accept(tb), 200)
	expect(request("POST", "/api/invitations", `{"username":"evil","role":"admin"}`, "alice"), 403)
	expect(request("POST", "/api/serverconf", `{"name":"Alice","address":"alice.example:28960"}`, "alice"), 200)
	expect(request("POST", "/api/serverconf", `{"name":"Bob","address":"bob.example:28960"}`, "bob"), 200)
	var owner string
	st.DB.QueryRow(`SELECT owner FROM servers WHERE id=2`).Scan(&owner)
	if owner != "alice" {
		t.Fatal(owner)
	}
	w := request("GET", "/api/serverconf", "", "alice")
	expect(w, 200)
	if strings.Contains(w.Body.String(), "bob.example") || strings.Contains(w.Body.String(), "legacy.example") {
		t.Fatal("cross tenant config", w.Body)
	}
	expect(request("PUT", "/api/serverconf/3", `{"name":"hijack","address":"bob.example:28960"}`, "alice"), 403)
	expect(request("POST", "/api/servertest", `{"id":3}`, "alice"), 403)
	expect(request("POST", "/api/serverconf", `{"name":"local","address":"a.example:28960","log_dir":"/gamelogs/"}`, "alice"), 403)
	expect(request("GET", "/api/servers/3/logs", "", "alice"), 403)
	expect(request("GET", "/api/servers/3/status", "", "alice"), 200)
	expect(request("GET", "/api/servers/1/logs", "", "legacy"), 200)
	expect(request("GET", "/api/servers/2/logs", "", "legacy"), 403)
	expect(request("GET", "/api/users", "", "alice"), 403)
	expect(request("GET", "/api/vps", "", "alice"), 403)
	expect(request("GET", "/metrics", "", "alice"), 403)
	tm := invite("mike", "moderator", "alice", 2, []string{"maps"})
	expect(accept(tm), 200)
	expect(request("POST", "/api/servers/2/rcon", `{"command":"quit"}`, "mike"), 403)
	expect(request("POST", "/api/servers/2/rcon", `{"command":"map mp_crash;quit"}`, "mike"), 403)
	expect(request("POST", "/api/servers/2/rcon", `{"command":"map mp_crash"}`, "mike"), 404) // authorized, but no RCON configured
	expect(request("POST", "/api/servers/3/rcon", `{"command":"map mp_crash"}`, "mike"), 403)
	expect(request("GET", "/api/servers/2/logs", "", "mike"), 403)
	oldInvite := invite("mike", "moderator", "alice", 2, []string{"rcon"})
	expect(request("PUT", "/api/servers/2/team/mike", `{"permissions":["private","kick"]}`, "alice"), 200)
	expect(accept(oldInvite), 400)
	expect(request("GET", "/api/servers/2/logs", "", "mike"), 200)
	if _, err := st.DB.Exec(`INSERT INTO cvar_history(server_id,ts,vars) VALUES(2,1,'{"rcon_password":"old-secret-value","mapname":"mp_crash"}')`); err != nil {
		t.Fatal(err)
	}
	cv := request("GET", "/api/servers/2/cvars", "", "mike")
	expect(cv, 200)
	if strings.Contains(cv.Body.String(), "old-secret-value") || !strings.Contains(cv.Body.String(), "mp_crash") {
		t.Fatal("cvars redaction", cv.Body)
	}
	expect(request("PUT", "/api/servers/2/team/mike", `{"permissions":["rcon"]}`, "bob"), 403)
	expect(request("PUT", "/api/servers/2/team/mike", `{"permissions":["unknown"]}`, "alice"), 400)
	expect(request("DELETE", "/api/servers/2/team/mike", "", "alice"), 200)
	expect(request("GET", "/api/servers/2/logs", "", "mike"), 403)
	tr := invite("revoke", "moderator", "alice", 2, nil)
	expect(request("DELETE", "/api/invitations/"+tokenHash(tr), "", "alice"), 200)
	expect(accept(tr), 400)
	te := invite("expired", "moderator", "alice", 2, nil)
	st.DB.Exec(`UPDATE invitations SET expires_at=1 WHERE token_hash=?`, tokenHash(te))
	expect(accept(te), 400)
	tt := invite("transfer", "moderator", "alice", 2, []string{"private"})
	expect(request("DELETE", "/api/users/alice", "", "root"), 409)
	expect(request("PUT", "/api/servers/2/owner", `{"username":"bob"}`, "alice"), 403)
	expect(request("PUT", "/api/servers/2/owner", `{"username":"bob"}`, "root"), 200)
	expect(accept(tt), 400)
	expect(request("GET", "/api/servers/2/logs", "", "alice"), 403)
	expect(request("GET", "/api/servers/2/logs", "", "bob"), 200)
	expect(request("GET", "/api/me", "", "bob"), 200)
	expect(request("POST", "/api/invitations", `{"username":"visitor","role":"admin"}`, ""), 401)
	// Migration is not re-applied at restart: no access to newly added servers.
	Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123"})
	if grants(st.DB, "legacy", 2)["private"] {
		t.Fatal("migration granted new server")
	}
}

func TestPrivateStreamRechecksAccess(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "stream.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	st.SaveServerConf(store.ServerConf{Name: "A", Host: "a.example", Port: 1, Owner: "alice"})
	st.DB.Exec(`INSERT INTO users(username,pass_hash,account_role) VALUES('mike','unused','moderator')`)
	st.DB.Exec(`INSERT INTO server_members VALUES(1,'mike','["private"]')`)
	token := strings.Repeat("x", 64)
	st.DB.Exec(`INSERT INTO sessions VALUES(?,?,?,?)`, tokenHash(token), "mike", time.Now().Unix(), time.Now().Add(time.Hour).Unix())
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/api/events", nil)
	c.Request.Header.Set("Cookie", sessionCookie+"="+token)
	c.Set("loginmode", true)
	c.Set("user", "mike")
	c.Set("account_role", "moderator")
	event := []byte(`{"type":"chat","server_id":1,"chat":{"message":"private"}}`)
	if !eventAllowed(st.DB, c, event) {
		t.Fatal("authorized stream denied")
	}
	if eventAllowed(st.DB, c, []byte(`{"type":"chat","server_id":2}`)) {
		t.Fatal("other server leaked")
	}
	st.DB.Exec(`DELETE FROM server_members WHERE server_id=1`)
	if eventAllowed(st.DB, c, event) {
		t.Fatal("revoked membership still streams")
	}
	st.DB.Exec(`INSERT INTO server_members VALUES(1,'mike','["private"]')`)
	st.DB.Exec(`DELETE FROM sessions`)
	if eventAllowed(st.DB, c, event) {
		t.Fatal("expired session still streams")
	}
	if !eventAllowed(st.DB, c, []byte(`{"type":"status","server_id":1}`)) {
		t.Fatal("public event denied")
	}
}
