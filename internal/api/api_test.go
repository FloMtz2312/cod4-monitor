package api

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/store"
)

func TestOccupancy(t *testing.T) {
	conns := [][2]int64{
		{0, 100},   // whole bucket
		{50, 80},   // overlaps the next one: peak 2 then 3
		{60, 200},  // continues after the bucket
		{80, 90},   // joins when [50,80) leaves: no extra peak
		{100, 150}, // after the bucket
	}
	avg, peak := occupancy(conns, 0, 100)
	if peak != 3 || avg != (100+30+40+10)/100.0 {
		t.Fatalf("avg=%v peak=%d", avg, peak)
	}
	if avg, peak := occupancy(nil, 0, 100); avg != 0 || peak != 0 {
		t.Fatalf("empty: %v %d", avg, peak)
	}
}

func TestSameOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/x", sameOrigin, func(c *gin.Context) { c.Status(200) })
	for _, tc := range []struct {
		host, origin, ctype string
		want                int
	}{
		{"localhost:8090", "http://localhost:8090", "application/json", 200},
		{"127.0.0.1:8090", "", "application/json", 200},
		{"localhost:8090", "https://evil.example", "application/json", 403},          // other site
		{"localhost:8090", "", "text/plain", 403},                                    // form/simple request, no preflight
		{"evil.example:8090", "http://evil.example:8090", "application/json", 403},   // DNS rebinding
		{"192.168.1.20:8090", "http://192.168.1.20:8090", "application/json", 403}, // LAN without password
	} {
		req := httptest.NewRequest("POST", "/x", strings.NewReader("{}"))
		req.Host = tc.host
		req.Header.Set("Content-Type", tc.ctype)
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != tc.want {
			t.Errorf("%+v: got %d", tc, w.Code)
		}
	}
}

func TestLoginSessions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	r.Use(proxySecurity(r, []string{"192.0.2.1/32"}))
	authRoutes(r, st.DB, "Babou", "s3cret")
	r.GET("/api/x", func(c *gin.Context) { c.String(200, c.GetString("user")) })
	r.GET("/api/servers/:id/status", func(c *gin.Context) { c.String(200, role(c)) })
	r.GET("/api/servers/:id/logs", func(c *gin.Context) { c.String(200, "logs") })
	r.POST("/api/servers/:id/rcon", sameOrigin, func(c *gin.Context) { c.String(200, "rcon") })
	r.NoRoute(func(c *gin.Context) { c.String(200, "page") })
	host := "monitor.example.ts.net"
	do := func(method, path, body, cookie string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Host = host
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", "https://"+host) // behind the HTTPS tunnel
		req.Header.Set("X-Forwarded-Proto", "https")
		if cookie != "" {
			req.Header.Set("Cookie", cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	login := func(u, p string) string {
		w := do("POST", "/api/login", `{"username":"`+u+`","password":"`+p+`"}`, "")
		if w.Code != 200 {
			return ""
		}
		c := w.Result().Cookies()[0]
		if !c.HttpOnly || !c.Secure {
			t.Errorf("cookie must be HttpOnly + Secure over https: %+v", c)
		}
		return c.Name + "=" + c.Value
	}
	if w := do("GET", "/api/x", "", ""); w.Code != 401 {
		t.Fatalf("api without session: %d", w.Code)
	}
	// visitor: the page and the public stats, nothing else
	if w := do("GET", "/", "", ""); w.Code != 200 {
		t.Fatalf("page without session: %d", w.Code)
	}
	if w := do("GET", "/api/servers/1/status", "", ""); w.Code != 200 || w.Body.String() != "visitor" {
		t.Fatalf("visitor stats: %d %s", w.Code, w.Body)
	}
	for _, x := range [][2]string{{"GET", "/api/servers/1/logs"}, {"POST", "/api/servers/1/rcon"}, {"GET", "/api/users"}} {
		if w := do(x[0], x[1], "{}", ""); w.Code != 401 {
			t.Fatalf("visitor %v: %d", x, w.Code)
		}
	}
	if login("babou", "wrong") != "" {
		t.Fatal("wrong password accepted")
	}
	admin := login("BABOU", "s3cret") // user names are case-insensitive
	if admin == "" || do("GET", "/api/x", "", admin).Body.String() != "babou" {
		t.Fatal("admin login failed")
	}
	// Accounts now join through a scoped invitation.
	if _, err := st.SaveServerConf(store.ServerConf{Name: "Test", Host: "example.org", Port: 28960, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	w := do("POST", "/api/invitations", `{"username":"Omni","role":"moderator","server_id":1,"permissions":["private"]}`, admin)
	if w.Code != 201 {
		t.Fatalf("invite: %d %s", w.Code, w.Body)
	}
	var invite struct{ Token string }
	json.Unmarshal(w.Body.Bytes(), &invite)
	if w = do("POST", "/api/invitations/accept", `{"token":"`+invite.Token+`","password":"longenough1"}`, ""); w.Code != 200 {
		t.Fatalf("accept: %d %s", w.Code, w.Body)
	}
	omni := login("omni", "longenough1")
	if omni == "" {
		t.Fatal("new account can't log in")
	}
	if w := do("GET", "/api/users", "", omni); w.Code != 403 {
		t.Fatalf("non admin listed users: %d", w.Code)
	}
	// semi-admin: reads the console, sends no command; the admin does both
	if w := do("GET", "/api/servers/1/logs", "", omni); w.Code != 200 {
		t.Fatalf("semi logs: %d", w.Code)
	}
	if w := do("GET", "/api/servers/1/status", "", omni); w.Body.String() != "semi" {
		t.Fatalf("semi role: %s", w.Body)
	}
	if w := do("POST", "/api/servers/1/rcon", "{}", omni); w.Code != 403 {
		t.Fatalf("semi sent rcon: %d", w.Code)
	}
	if w := do("POST", "/api/servers/1/rcon", "{}", admin); w.Code != 200 {
		t.Fatalf("admin rcon: %d", w.Code)
	}
	// deleting the account ends its sessions
	if w := do("DELETE", "/api/users/omni", "", admin); w.Code != 200 {
		t.Fatalf("delete: %d", w.Code)
	}
	if w := do("GET", "/api/x", "", omni); w.Code != 401 {
		t.Fatalf("deleted account still logged in: %d", w.Code)
	}
	// brute force: blocked after 10 failures from the same IP
	for range 10 {
		login("babou", "nope")
	}
	if w := do("POST", "/api/login", `{"username":"babou","password":"s3cret"}`, ""); w.Code != 429 {
		t.Fatalf("no lockout after 10 failures: %d", w.Code)
	}
}

func TestPassword(t *testing.T) {
	h, err := hashPassword("hunter22")
	if err != nil || !checkPassword("hunter22", h) || checkPassword("hunter23", h) || checkPassword("hunter22", "garbage") {
		t.Fatalf("hash %q err %v", h, err)
	}
}

func TestRecords(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	base := time.Now().Add(-time.Hour)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }
	kill := func(s int, killer, victim, cause string) {
		if err := st.AddKill(1, at(s), killer, killer, killer, "axis", victim, victim, victim, "allies", "m40a3_mp", cause, "head", 100); err != nil {
			t.Fatal(err)
		}
	}
	st.StartRound(1, at(0), "mp_crash", "sd")
	kill(1, "a", "b", "MOD_HEAD_SHOT")
	kill(2, "a", "c", "MOD_RIFLE_BULLET")
	kill(3, "a", "d", "MOD_HEAD_SHOT") // a: 3 in a row
	st.StartRound(1, at(10), "mp_crash", "sd")
	kill(11, "a", "b", "MOD_RIFLE_BULLET") // new round: streak back to 1, same match (same map)
	kill(12, "b", "a", "MOD_RIFLE_BULLET")
	st.StartRound(1, at(20), "mp_bog", "sd") // new match
	kill(21, "b", "a", "MOD_RIFLE_BULLET")

	r := gin.New()
	featureRoutes(r.Group("/s/:id"), st, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/s/1/records?range=24h", nil))
	var res struct {
		Records map[string]struct {
			Key   string
			Value int
		}
		Day struct{ Key string } `json:"day"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatal(err, w.Body.String())
	}
	want := map[string]int{"streak": 3, "round_kills": 3, "match_kills": 4, "round_headshots": 2, "match_headshots": 2}
	for k, v := range want {
		if got := res.Records[k]; got.Value != v || got.Key != "a" {
			t.Errorf("%s = %+v, want a=%d", k, got, v)
		}
	}
	if res.Day.Key != "a" {
		t.Errorf("player of the day = %q", res.Day.Key)
	}
}
