package api

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/store"
)

func TestAdminChat(t *testing.T) {
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
	// 1 = moderator (20), 2 = admin (40), 3 = simple user
	raw.Exec(`INSERT INTO clients (id, guid, name, group_bits, time_add, time_edit) VALUES (1, 'g1', '^1Babou', 8, 1, 1), (2, 'g2', 'Omni Max', 16, 1, 1), (3, 'g3', 'Pleb', 1, 1, 1)`)
	raw.Close()

	st, err := store.Open(filepath.Join(dir, "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := st.SaveServerConf(store.ServerConf{Name: "S", Host: "127.0.0.1", Port: 28960, Enabled: true, Logs: store.LogFiles{B3DB: dbPath}})
	for _, u := range []string{"babou", "omni", "pleb", "stranger"} {
		st.DB.Exec(`INSERT INTO users(username, pass_hash, account_role) VALUES(?, 'x', 'moderator')`, u)
	}
	for _, u := range []string{"babou", "omni", "pleb"} {
		st.DB.Exec(`INSERT INTO server_members VALUES(?, ?, '["reports"]')`, id, u)
	}

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("loginmode", true); c.Set("user", c.GetHeader("X-User")) })
	adminChatRoutes(r.Group("/api/servers/:id"), st, "boss", nil)
	base := "/api/servers/" + itoa(id) + "/adminchat"
	do := func(user, method, path, body string) (int, map[string]any) {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-User", user)
		req.Host = "localhost"
		r.ServeHTTP(w, req)
		var out map[string]any
		json.Unmarshal(w.Body.Bytes(), &out)
		return w.Code, out
	}
	link := func(user, guid, name string) {
		t.Helper()
		code, out := do(user, "POST", base+"/link", "{}")
		if code != 200 {
			t.Fatalf("link code %s: %d %v", user, code, out)
		}
		if u, ok, err := st.UseAdminLinkCode(id, strings.ToLower(out["code"].(string)), guid, name, time.Now()); err != nil || !ok || u != user {
			t.Fatalf("!link %s: %q %v %v", user, u, ok, err)
		}
	}

	if code, _ := do("stranger", "GET", base, ""); code != 403 {
		t.Fatalf("not a member of the server: %d", code)
	}
	if code, out := do("babou", "GET", base, ""); code != 200 || out["linked"] != false || out["admin"] != false || out["messages"] != nil {
		t.Fatalf("not linked yet: %d %v", code, out)
	}
	if _, ok, _ := st.UseAdminLinkCode(id, "ZZZZZZ", "g1", "x", time.Now()); ok {
		t.Fatal("unknown code accepted")
	}
	link("babou", "g1", "^1Babou")
	link("omni", "g2", "Omni Max")
	link("pleb", "g3", "Pleb")
	if code, out := do("pleb", "POST", base, `{"body":"hello"}`); code != 403 {
		t.Fatalf("B3 user (level 1) may not write: %d %v", code, out)
	}
	if code, out := do("pleb", "GET", base, ""); code != 200 || out["linked"] != true || out["admin"] != false {
		t.Fatalf("pleb state: %d %v", code, out)
	}
	code, out := do("babou", "GET", base+"?fresh=1", "")
	if code != 200 || out["admin"] != true || len(out["admins"].([]any)) != 2 {
		t.Fatalf("admins: %d %v", code, out)
	}

	// @OmniMax = pseudo of omni (spaces removed); @babou (the author) is not notified
	if code, out := do("babou", "POST", base, `{"body":"  salut @OmniMax, regarde ça @babou\u0007 "}`); code != 200 {
		t.Fatalf("send: %d %v", code, out)
	} else if m := out["message"].(map[string]any); m["body"] != "salut @OmniMax, regarde ça @babou" || len(m["mentions"].([]any)) != 1 || m["mentions"].([]any)[0] != "omni" {
		t.Fatalf("message: %v", m)
	}
	if _, out := do("pleb", "GET", base+"/unread", ""); out["admin"] != false {
		t.Fatalf("pleb in the chat: %v", out)
	}
	if _, out := do("omni", "GET", base+"/unread", ""); out["admin"] != true || out["unread"] != float64(1) || out["mentions"] != float64(1) {
		t.Fatalf("omni unread: %v", out)
	}
	if _, out := do("babou", "GET", base+"/unread", ""); out["unread"] != float64(0) {
		t.Fatalf("own message unread: %v", out)
	}
	_, out = do("omni", "GET", base, "")
	msgs := out["messages"].([]any)
	last := msgs[len(msgs)-1].(map[string]any)["id"].(float64)
	if code, _ := do("omni", "POST", base+"/read", `{"id":`+itoa(int64(last))+`}`); code != 200 {
		t.Fatal("read")
	}
	if _, out := do("omni", "GET", base+"/unread", ""); out["unread"] != float64(0) || out["mentions"] != float64(0) {
		t.Fatalf("after read: %v", out)
	}
	if _, out := do("omni", "GET", base+"?after="+itoa(int64(last)), ""); len(out["messages"].([]any)) != 0 {
		t.Fatalf("after: %v", out)
	}

	// unlinked: no longer in the chat
	if code, _ := do("omni", "DELETE", base+"/link", "{}"); code != 200 {
		t.Fatal("unlink")
	}
	if code, _ := do("omni", "POST", base, `{"body":"x"}`); code != 403 {
		t.Fatalf("unlinked account wrote: %d", code)
	}

	// without B3: the team of the server (owner and members), no link, mentions by account name
	other, _ := st.SaveServerConf(store.ServerConf{Name: "No B3", Host: "127.0.0.1", Port: 28961, Enabled: true, Owner: "omni"})
	st.DB.Exec(`UPDATE servers SET owner='omni' WHERE id=?`, other)
	st.DB.Exec(`UPDATE users SET account_role='admin' WHERE username='omni'`)
	st.DB.Exec(`INSERT INTO server_members VALUES(?, 'pleb', '["kick"]')`, other)
	nb := "/api/servers/" + itoa(other) + "/adminchat"
	if code, out := do("pleb", "GET", nb, ""); code != 200 || out["admin"] != true || out["b3"] != false || len(out["admins"].([]any)) != 2 {
		t.Fatalf("team chat: %d %v", code, out)
	}
	if code, _ := do("babou", "GET", nb, ""); code != 403 {
		t.Fatalf("not in the team: %d", code)
	}
	if code, _ := do("pleb", "POST", nb+"/link", "{}"); code != 400 {
		t.Fatalf("link code without B3: %d", code)
	}
	if code, out := do("pleb", "POST", nb, `{"body":"@omni regarde"}`); code != 200 || out["message"].(map[string]any)["mentions"].([]any)[0] != "omni" {
		t.Fatalf("team mention: %d %v", code, out)
	}
	if _, out := do("omni", "GET", nb+"/unread", ""); out["mentions"] != float64(1) {
		t.Fatalf("owner unread: %v", out)
	}
	if code, _ := do("boss", "POST", nb, `{"body":"hello"}`); code != 200 {
		t.Fatalf("super admin without B3: %d", code)
	}
}

func TestMentioned(t *testing.T) {
	admins := withTags([]ChatAdmin{{Username: "babou", Name: "^1Babou", B3ID: 1}, {Username: "omni", Name: "^3UpT# Omni Max", B3ID: 12},
		{Username: "zed", Name: "Zed", B3ID: 3}, {Username: "zed2", Name: "^2zed", B3ID: 4}})
	if tags := []string{admins[0].Tag, admins[1].Tag, admins[2].Tag, admins[3].Tag}; !slices.Equal(tags, []string{"babou", "uptomnimax", "zed", "zed2"}) {
		t.Fatalf("tags: %v", tags) // the second "zed" gets its account name
	}
	for _, c := range []struct {
		body string
		want []string
	}{
		{"@UpTOmniMax viens", []string{"omni"}},
		{"@babou, @omni et @ZED.", []string{"omni", "zed"}}, // @babou is the author; account names work too
		{"@zed2", []string{"zed2"}},
		{"mail@zed.fr", nil},
		{"@all urgent", []string{"omni", "zed", "zed2"}},
		{"@12 @inconnu", nil}, // B3 numbers don't mention
	} {
		if got := mentioned(c.body, "babou", admins); !slices.Equal(got, c.want) {
			t.Errorf("%q: %v, want %v", c.body, got, c.want)
		}
	}
}
