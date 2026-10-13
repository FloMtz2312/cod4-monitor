package api

import (
	"bytes"
	"cod4monitor/internal/store"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image/png"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func TestTrustedProxyHeaders(t *testing.T) {
	r := gin.New()
	r.Use(proxySecurity(r, []string{"10.0.0.2/32"}))
	r.GET("/", func(c *gin.Context) { c.JSON(200, gin.H{"ip": clientIP(c), "secure": secureRequest(c)}) })
	for _, tc := range []struct {
		peer, xff, want string
		secure          bool
	}{
		{"203.0.113.9:5000", "1.2.3.4", "203.0.113.9", false},
		{"10.0.0.2:5000", "1.2.3.4, 203.0.113.9", "203.0.113.9", true},
		{"10.0.0.2:5000", "invalid", "10.0.0.2", true},
	} {
		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = tc.peer
		req.Header.Set("X-Forwarded-For", tc.xff)
		req.Header.Set("X-Forwarded-Proto", "https")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		var got struct {
			IP     string `json:"ip"`
			Secure bool
		}
		json.Unmarshal(w.Body.Bytes(), &got)
		if got.IP != tc.want || got.Secure != tc.secure {
			t.Fatalf("%+v got %+v", tc, got)
		}
	}
}
func TestPasswordChangeRevokesSessions(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	h, _ := hashPassword("firstpassword")
	s.DB.Exec(`INSERT INTO users(username,pass_hash) VALUES('reader',?)`, h)
	s.DB.Exec(`INSERT INTO sessions VALUES('token','reader',0,9999999999)`)
	if e = setPassword(s.DB, "reader", "newpassword"); e != nil {
		t.Fatal(e)
	}
	var n int
	s.DB.QueryRow(`SELECT COUNT(*) FROM sessions`).Scan(&n)
	if n != 0 {
		t.Fatal("sessions retained")
	}
	if checkPassword("x", "pbkdf2-sha256$99999999999$AA$AA") {
		t.Fatal("bad hash accepted")
	}
}
func TestExperienceIntegration(t *testing.T) {
	s, e := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	id, _ := s.UpsertServer("Test community", "127.0.0.1", 28960)
	now := time.Now().Add(-time.Minute)
	s.StartRound(id, now.Add(-time.Minute), "mp_crash", "sd")
	s.AddKill(id, now, "a", "Alice", "Alice", "axis", "b", "Bob", "Bob", "allies", "ak47_mp", "MOD_HEAD_SHOT", "head", 100)
	s.AddGameEvent(id, now.Add(time.Second), "plant", "a", "Alice", "Alice", 0)
	s.AddAccuracy(id, now, "a", "Alice", "Alice", 10, 4, 100, 0, 0, 0)
	r := Router(Config{Store: s, Hub: NewHub(), UI: fstest.MapFS{"index.html": &fstest.MapFile{Data: []byte("ok")}}, ServerLoc: time.UTC, User: "admin", Password: "password123"})
	for _, path := range []string{"/api/servers/1/form?key=a", "/api/servers/1/challenges", "/api/servers/1/match?id=1", "/api/servers/1/card.png?type=player&key=a", "/api/servers/1/card.png?type=match&match=1"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 200 {
			t.Fatalf("%s: %d %s", path, w.Code, w.Body.String())
		}
		if strings.Contains(path, "card.png") {
			img, e := png.Decode(bytes.NewReader(w.Body.Bytes()))
			if e != nil || img.Bounds().Dx() != 1200 || img.Bounds().Dy() != 630 {
				t.Fatalf("bad PNG %v", e)
			}
		}
		if strings.Contains(path, "match?id") {
			var d struct{ Timeline []map[string]any }
			json.Unmarshal(w.Body.Bytes(), &d)
			if len(d.Timeline) != 2 {
				t.Fatalf("timeline: %s", w.Body.String())
			}
		}
	}
	for _, path := range []string{"/api/observability", "/metrics"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatalf("public admin endpoint %s: %d", path, w.Code)
		}
	}
	req := httptest.NewRequest("PUT", "/api/servers/1/challenges", strings.NewReader(`{"kills":1,"headshots":1,"objectives":1}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 401 {
		t.Fatalf("visitor changed targets: %d", w.Code)
	}
	f, e := playerForm(s.DB, "1", "a", now.Add(-24*time.Hour).Unix(), time.Now().Unix())
	if e != nil || f.Kills != 1 || f.Headshots != 1 || f.Accuracy == nil || *f.Accuracy != 40 {
		t.Fatalf("form %+v %v", f, e)
	}
}
