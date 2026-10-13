package api

import (
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestAppHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	built := fstest.MapFS{
		"index.html":        {Data: []byte("<html>app</html>")},
		"assets/app-1.js":   {Data: []byte("console.log(1)")},
		"maps/mp_crash.jpg": {Data: []byte("jpg")},
		"sw.js":             {Data: []byte("self.onpush=1")},
		".gitkeep":          {Data: nil},
	}
	empty := fstest.MapFS{".gitkeep": {Data: nil}}
	cases := []struct {
		fs                  fstest.MapFS
		method, path        string
		code                int
		body, cache, header string
	}{
		{built, "GET", "/", 200, "<html>app</html>", "no-cache", ""},
		{built, "GET", "/index.html", 200, "<html>app</html>", "no-cache", ""},
		{built, "GET", "/login.html", 200, "<html>app</html>", "no-cache", ""},  // old sign-in page: the app's screen
		{built, "GET", "/invite.html", 200, "<html>app</html>", "no-cache", ""}, // old invitation links (token after #)
		{built, "GET", "/v2", 301, "", "", "/"},                                 // the address while it was being built
		{built, "GET", "/v2/", 301, "", "", "/"},
		{built, "GET", "/v2/?s=2", 301, "", "", "/?s=2"}, // a notification tapped: the server is kept
		{built, "GET", "/v2/joueur/abc", 301, "", "", "/"},
		{built, "GET", "/assets/app-1.js", 200, "console.log(1)", "public, max-age=31536000, immutable", ""},
		{built, "GET", "/maps/mp_crash.jpg", 200, "jpg", "public, max-age=86400", ""},
		{built, "GET", "/sw.js", 200, "self.onpush=1", "no-cache", ""},
		{built, "GET", "/assets/missing.js", 404, "", "", ""}, // an old asset: no page in place of a script
		{built, "GET", "/wp-login.php", 404, "", "", ""},
		{built, "GET", "/../../etc/passwd", 404, "", "", ""}, // cleaned to /etc/passwd: not a file of the app
		{built, "POST", "/", 405, "", "", ""},
		{empty, "GET", "/", 503, "Interface non compilée : cd ui && npm ci && npm run build", "", ""},
	}
	for _, tc := range cases {
		r := gin.New()
		r.NoRoute(appHandler(tc.fs))
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.code {
			t.Errorf("%s %s: code %d, want %d", tc.method, tc.path, w.Code, tc.code)
		}
		if tc.body != "" && w.Body.String() != tc.body {
			t.Errorf("%s %s: body %q", tc.method, tc.path, w.Body.String())
		}
		if tc.cache != "" && w.Header().Get("Cache-Control") != tc.cache {
			t.Errorf("%s %s: Cache-Control %q", tc.method, tc.path, w.Header().Get("Cache-Control"))
		}
		if tc.header != "" && w.Header().Get("Location") != tc.header {
			t.Errorf("%s %s: Location %q", tc.method, tc.path, w.Header().Get("Location"))
		}
	}
}
