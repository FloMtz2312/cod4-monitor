package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Only an explicitly trusted direct peer can describe the original TLS connection.
// Gin validates X-Forwarded-For from right to left against the same allowlist.
func proxySecurity(r *gin.Engine, proxies []string) gin.HandlerFunc {
	if err := r.SetTrustedProxies(proxies); err != nil {
		panic("invalid TRUSTED_PROXIES: " + err.Error())
	}
	r.RemoteIPHeaders = []string{"X-Forwarded-For"}
	var networks []*net.IPNet
	for _, p := range proxies {
		if ip := net.ParseIP(p); ip != nil {
			bits := 128
			if ip.To4() != nil {
				bits = 32
			}
			networks = append(networks, &net.IPNet{IP: ip, Mask: net.CIDRMask(bits, bits)})
			continue
		}
		_, n, _ := net.ParseCIDR(p)
		if n != nil {
			networks = append(networks, n)
		}
	}
	return func(c *gin.Context) {
		host, _, _ := net.SplitHostPort(c.Request.RemoteAddr)
		for _, n := range networks {
			if n.Contains(net.ParseIP(host)) {
				c.Set("trusted_https", c.GetHeader("X-Forwarded-Proto") == "https")
				break
			}
		}
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("Referrer-Policy", "same-origin")
		c.Header("X-Frame-Options", "DENY")
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Header("Cache-Control", "no-store")
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	}
}
