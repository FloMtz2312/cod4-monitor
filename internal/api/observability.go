package api

import (
	"cod4monitor/internal/store"
	"fmt"
	"github.com/gin-gonic/gin"
	"strconv"
	"time"
)

func observabilityRoutes(r *gin.Engine, st *store.Store) {
	r.GET("/api/observability", func(c *gin.Context) {
		h, err := rows(st.DB, `SELECT server_id,file,last_success,backlog_bytes,lines,rejected,errors,rotations,missing FROM log_health WHERE ? OR server_id IN (SELECT id FROM servers WHERE owner=?) ORDER BY server_id,file`, super(c), c.GetString("user"))
		if err != nil {
			c.JSON(500, gin.H{"error": "lecture des métriques impossible"})
			return
		}
		out := gin.H{"logs": h, "now": time.Now().Unix()}
		if super(c) {
			out["backup_verified_at"] = st.Meta("backup_verified_at")
			out["backup_error"] = st.Meta("backup_error")
		}
		c.JSON(200, out)
	})
}
func logMetrics(c *gin.Context, st *store.Store) {
	rs, err := rows(st.DB, `SELECT server_id,file,last_success,backlog_bytes,lines,rejected,errors,rotations,missing FROM log_health`)
	if err != nil {
		return
	}
	for _, m := range []struct{ name, col, kind string }{
		{"cod4_log_last_success_seconds", "last_success", "gauge"}, {"cod4_log_backlog_bytes", "backlog_bytes", "gauge"},
		{"cod4_log_lines_total", "lines", "counter"}, {"cod4_log_rejected_total", "rejected", "counter"},
		{"cod4_log_errors_total", "errors", "counter"}, {"cod4_log_rotations_total", "rotations", "counter"}, {"cod4_log_missing", "missing", "gauge"},
	} {
		fmt.Fprintf(c.Writer, "# TYPE %s %s\n", m.name, m.kind)
		for _, r := range rs {
			fmt.Fprintf(c.Writer, "%s{server_id=\"%v\",file=%q} %v\n", m.name, r["server_id"], r["file"], r[m.col])
		}
	}
	v, _ := strconv.ParseInt(st.Meta("backup_verified_at"), 10, 64)
	fmt.Fprintf(c.Writer, "# TYPE cod4_backup_verified_seconds gauge\ncod4_backup_verified_seconds %d\n", v)
}
