package api

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
)

// appHandler serves the interface (ui/, Svelte, built into ui/dist): one page, whose hashed assets are cached for a
// year, and its static files (fonts, map images, icons, service worker). The old addresses still lead to it: /v2/...
// (the address while it was being built) is redirected to /, and /login.html and /invite.html get the page, which
// opens its sign-in or invitation screen.
func appHandler(dist fs.FS) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusMethodNotAllowed)
			return
		}
		p := c.Request.URL.Path
		if p == "/v2" || strings.HasPrefix(p, "/v2/") { // the fragment (#/screen) is kept by the browser
			to := "/"
			if q := c.Request.URL.RawQuery; q != "" {
				to += "?" + q
			}
			c.Redirect(http.StatusMovedPermanently, to)
			return
		}
		name := strings.TrimPrefix(path.Clean(p), "/")
		switch name {
		case "", ".", "index.html", "login.html", "invite.html":
			page, err := fs.ReadFile(dist, "index.html")
			if err != nil {
				c.String(http.StatusServiceUnavailable, "Interface non compilée : cd ui && npm ci && npm run build")
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "text/html; charset=utf-8", page)
			return
		}
		if info, err := fs.Stat(dist, name); err == nil && !info.IsDir() {
			switch {
			case strings.HasPrefix(name, "assets/"):
				c.Header("Cache-Control", "public, max-age=31536000, immutable")
			case name == "sw.js" || name == "manifest.webmanifest":
				c.Header("Cache-Control", "no-cache") // same name, new content: checked at each load
			default:
				c.Header("Cache-Control", "public, max-age=86400") // fonts, map images, icons
			}
			http.ServeFileFS(c.Writer, c.Request, dist, name)
			return
		}
		c.Status(http.StatusNotFound)
	}
}
