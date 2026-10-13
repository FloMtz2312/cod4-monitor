package api

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/png"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/store"
)

func TestBrand(t *testing.T) {
	gin.SetMode(gin.TestMode)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	r := gin.New()
	brandRoutes(r.Group("/api/servers/:id"), st.DB)
	put := func(body string) int {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("PUT", "/api/servers/1/brand", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		req.Host = "localhost"
		r.ServeHTTP(w, req)
		return w.Code
	}
	var buf bytes.Buffer
	png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 64, 64)))
	logo := "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())

	if code := put(`{"accent":"#3e8bff","logo":"` + logo + `"}`); code != 200 {
		t.Fatalf("save: %d", code)
	}
	var accent string
	var ts int64
	st.DB.QueryRow(`SELECT accent, logo_ts FROM servers WHERE id=1`).Scan(&accent, &ts)
	if accent != "#3E8BFF" || ts == 0 {
		t.Fatalf("saved: %q %d", accent, ts)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/servers/1/logo.png", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || !bytes.Equal(w.Body.Bytes(), buf.Bytes()) {
		t.Fatalf("logo: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	svg := "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(`<svg onload="alert(1)"/>`))
	for _, bad := range []string{`{"accent":"red"}`, `{"logo":"` + svg + `"}`, `{"logo":"data:image/png;base64,aGVsbG8="}`} {
		if code := put(bad); code != 400 {
			t.Fatalf("%s accepted: %d", bad, code)
		}
	}
	// default color, logo kept; then logo removed
	if put(`{"accent":""}`) != 200 || put(`{"accent":"","logo":""}`) != 200 {
		t.Fatal("reset refused")
	}
	var a, l any
	st.DB.QueryRow(`SELECT accent, logo FROM servers WHERE id=1`).Scan(&a, &l)
	if a != nil || l != nil {
		t.Fatalf("reset: %v %v", a, l)
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/servers/1/logo.png", nil))
	if w.Code != 404 {
		t.Fatalf("removed logo: %d", w.Code)
	}
}
