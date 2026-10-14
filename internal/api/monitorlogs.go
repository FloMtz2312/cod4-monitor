package api

import (
	"net"

	"github.com/gin-gonic/gin"
)

// monitorLogRoutes: the last lines of the monitor's own log (what "docker logs" shows, secrets masked), for the super
// admin only. Not in the tenancy allowlist, so other accounts get 403; without a dashboard password, local
// addresses only, like the commands.
func monitorLogRoutes(r *gin.Engine, logs func() []string) {
	r.GET("/api/monitor-logs", func(c *gin.Context) {
		if !super(c) || !c.GetBool("loginmode") && !localHost(c) {
			c.JSON(403, gin.H{"error": "réservé au super admin"})
			return
		}
		lines := []string{}
		if logs != nil {
			lines = logs()
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(200, gin.H{"lines": lines})
	})
}

func localHost(c *gin.Context) bool {
	host, _, err := net.SplitHostPort(c.Request.Host)
	if err != nil {
		host = c.Request.Host
	}
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}
