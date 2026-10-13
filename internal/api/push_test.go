package api

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/push"
	"cod4monitor/internal/store"
)

func browserKeys(t *testing.T) (p256dh, auth string) {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := make([]byte, 16)
	rand.Read(a)
	return pushB64.EncodeToString(k.PublicKey().Bytes()), pushB64.EncodeToString(a)
}

func TestPushSubscriptions(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "push.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	st.SaveServerConf(store.ServerConf{Name: "One", Host: "one.example", Port: 28960, Enabled: true})
	st.SaveServerConf(store.ServerConf{Name: "Two", Host: "two.example", Port: 28960, Enabled: true})
	ps, err := push.New(st, "https://monitor.example")
	if err != nil {
		t.Fatal(err)
	}
	r := Router(Config{Store: st, Hub: NewHub(), User: "root", Password: "password123", Push: ps})
	h, _ := hashPassword("password123") // after the router: the legacy migration gives every existing account "private"
	now := time.Now().Unix()
	st.DB.Exec(`INSERT INTO users(username,pass_hash,created_at,account_role) VALUES('mod',?,?,'moderator'),('alice',?,?,'admin')`, h, now, h, now)
	st.DB.Exec(`UPDATE servers SET owner='alice' WHERE id=2`)
	st.DB.Exec(`INSERT INTO server_members(server_id,username,permissions) VALUES(1,'mod','["reports"]')`)
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

	expect(request("GET", "/api/push", "", ""), 401) // no account, no notifications
	w := request("GET", "/api/push", "", "mod")
	expect(w, 200)
	var key struct{ Key string }
	json.Unmarshal(w.Body.Bytes(), &key)
	if key.Key != ps.PublicKey() || key.Key == "" {
		t.Fatalf("key %q", key.Key)
	}

	endpoint := "https://fcm.googleapis.com/fcm/send/mod-phone"
	pub, auth := browserKeys(t)
	sub := func(endpoint, pub, auth, types string) string {
		return `{"endpoint":"` + endpoint + `","keys":{"p256dh":"` + pub + `","auth":"` + auth + `"},"lang":"en","types":` + types + `,"muted":[2]}`
	}
	expect(request("PUT", "/api/push", sub("https://127.0.0.1/x", pub, auth, `["report"]`), "mod"), 400) // not a push service
	expect(request("PUT", "/api/push", sub(endpoint, "AAAA", auth, `["report"]`), "mod"), 400)
	expect(request("PUT", "/api/push", sub(endpoint, pub, auth, `["everything"]`), "mod"), 400)
	expect(request("PUT", "/api/push", sub(endpoint, pub, auth, `["report","down","report"]`), "mod"), 200)
	saved, ok, _ := st.PushSub(endpoint)
	if !ok || saved.Username != "mod" || saved.Lang != "en" || len(saved.Types) != 2 || len(saved.Muted) != 1 || saved.P256dh != pub {
		t.Fatalf("saved %+v", saved)
	}
	w = request("GET", "/api/push?endpoint="+endpoint, "", "mod")
	if !strings.Contains(w.Body.String(), `"sub":{`) {
		t.Fatalf("own subscription not returned: %s", w.Body)
	}
	if w = request("GET", "/api/push?endpoint="+endpoint, "", "alice"); strings.Contains(w.Body.String(), `"sub"`) {
		t.Fatalf("someone else's subscription returned: %s", w.Body)
	}
	expect(request("DELETE", "/api/push", `{"endpoint":"`+endpoint+`"}`, "alice"), 200)
	if _, ok, _ := st.PushSub(endpoint); !ok {
		t.Fatal("deleted by another account")
	}

	// rights at send time: mod handles the reports of server 1 only, alice owns server 2, root everything
	access := Access(st.DB, "Root", false)
	for _, c := range []struct {
		user   string
		server int64
		kind   string
		need   []string
		want   bool
	}{
		{"mod", 1, "report", []string{"reports", "private"}, true},
		{"mod", 1, "suspect", []string{"private"}, false},
		{"mod", 1, "down", nil, true},
		{"mod", 2, "down", nil, false},
		{"alice", 2, "b3ban", []string{"private"}, true},
		{"alice", 1, "down", nil, false},
		{"root", 1, "b3ban", []string{"private"}, true},
		{"ghost", 1, "down", nil, false},
	} {
		ps.Access = access
		s := store.PushSub{Username: c.user, Types: push.Kinds}
		if got := ps.Allowed(s, push.Message{Kind: c.kind, Server: c.server, Need: c.need}); got != c.want {
			t.Errorf("%s on %d, %s: %v", c.user, c.server, c.kind, got)
		}
	}

	expect(request("DELETE", "/api/push", `{"endpoint":"`+endpoint+`"}`, "mod"), 200)
	if _, ok, _ := st.PushSub(endpoint); ok {
		t.Fatal("not unsubscribed")
	}
}

// pushService: a fake push service that hands over each request.
type pushService chan *http.Request

func (p pushService) RoundTrip(r *http.Request) (*http.Response, error) {
	io.Copy(io.Discard, r.Body)
	p <- r
	return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func TestPushScan(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "scan.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	st.SaveServerConf(store.ServerConf{Name: "One", Host: "one.example", Port: 28960, Enabled: true})
	ps, _ := push.New(st, "https://monitor.example")
	got := make(pushService, 10)
	ps.Client = &http.Client{Transport: got}
	pub, auth := browserKeys(t)
	st.SavePushSub(store.PushSub{Endpoint: "https://fcm.googleapis.com/fcm/send/a", Username: "", P256dh: pub, Auth: auth, Lang: "fr",
		Types: []string{"report"}}, time.Now())
	now := time.Now()
	wait := func(n int) {
		t.Helper()
		for range n {
			select {
			case <-got:
			case <-time.After(5 * time.Second):
				t.Fatal("no push sent")
			}
		}
		select {
		case r := <-got:
			t.Fatalf("one push too many: %s", r.URL)
		case <-time.After(200 * time.Millisecond):
		}
	}

	st.AddReport(1, now.Add(-time.Hour), "a", "A", "A", "old", "before the monitor knew")
	if err := pushScan(st, ps, 1, "One", now); err != nil { // first scan: what is there is not news
		t.Fatal(err)
	}
	wait(0)
	st.AddReport(1, now, "a", "A", "A", "cheater", "aimbot")
	st.AddReport(1, now, "b", "B", "B", "other", "") // dismissed before the scan: not sent
	st.DB.Exec(`UPDATE reports SET handled_at=1, handled_by='x' WHERE target='other'`)
	pushScan(st, ps, 1, "One", now)
	wait(1)
	pushScan(st, ps, 1, "One", now) // announced once
	wait(0)

	m, ok := inboxPush(inboxItem{"id": "report:3", "type": "report", "reporter_name": "^1Babou", "target": "cheater", "reason": "aimbot"}, "One")
	if !ok || m.Kind != "report" || m.Title != "Signalement : cheater" || m.Body != "Babou : « aimbot » · One" || !m.Urgent {
		t.Fatalf("report message %+v", m)
	}
	m, _ = inboxPush(inboxItem{"id": "b3ban:9", "type": "b3ban", "name_raw": "Bad", "ban_type": "TempBan", "admin": "", "reason": ""}, "One")
	if m.Kind != "b3ban" || m.TitleEN != "Bad temp-banned" || m.Body != "Par B3 · One" || m.Need[0] != "private" {
		t.Fatalf("b3ban message %+v", m)
	}
	m, _ = inboxPush(inboxItem{"id": "suspect:k:1", "type": "suspect", "name_raw": "Ace", "why": []string{"hs", "kd"}, "hs_rate": 0.72,
		"server_hs_rate": 0.25, "kills": int64(80), "deaths": int64(10)}, "One")
	if m.Kind != "suspect" || m.Body != "72 % de headshots (les autres : 25 %) · K/D 8.0 sur 7 jours · One" {
		t.Fatalf("suspect message %+v", m)
	}
}
