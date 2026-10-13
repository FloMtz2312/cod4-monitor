package api

import (
	"database/sql"
	"log/slog"
	"strconv"
	"time"

	"cod4monitor/internal/query"
	"github.com/gin-gonic/gin"
)

// auditActions: the dashboard actions kept in admin_log (who did what, when), by route.
// Handlers add the details with c.Set("audit", ...).
var auditActions = map[string]string{
	"POST /api/invitations": "Invitation créée", "POST /api/invitations/accept": "Invitation acceptée", "DELETE /api/invitations/:token": "Invitation révoquée",
	"PUT /api/servers/:id/team/:name": "Permissions modifiées", "DELETE /api/servers/:id/team/:name": "Modérateur retiré", "PUT /api/servers/:id/owner": "Propriétaire changé",
	"PUT /api/servers/:id/challenges":             "Objectifs communautaires",
	"POST /api/servers/:id/rcon":                  "Commande rcon",
	"POST /api/servers/:id/kick":                  "Kick",
	"POST /api/servers/:id/ban":                   "Ban",
	"POST /api/servers/:id/unban":                 "Débannissement",
	"POST /api/servers/:id/b3/penalty":            "Sanction B3",
	"POST /api/servers/:id/b3/unban":              "Ban B3 levé",
	"POST /api/servers/:id/b3/penalty/:pid/lift":  "Sanction B3 levée",
	"POST /api/servers/:id/b3/group":              "Grade B3",
	"PUT /api/servers/:id/notes":                  "Note joueur",
	"POST /api/servers/:id/schedules":             "Commande programmée ajoutée",
	"POST /api/servers/:id/schedules/:sid/toggle": "Commande programmée en pause / reprise",
	"DELETE /api/servers/:id/schedules/:sid":      "Commande programmée supprimée",
	"PUT /api/servers/:id/welcome":                "Message de bienvenue",
	"POST /api/servers/:id/reports/:rid/done":     "Signalement traité",
	"POST /api/users":                             "Compte créé",
	"POST /api/serverconf":                        "Serveur ajouté",
	"PUT /api/serverconf/:sid":                    "Serveur modifié",
	"POST /api/serverconf/:sid/enabled":           "Surveillance d'un serveur activée / arrêtée",
	"POST /api/users/:name/password":              "Mot de passe réinitialisé",
	"DELETE /api/users/:name":                     "Compte supprimé",
	"POST /api/me/password":                       "Mot de passe changé",
	"POST /api/servers/:id/tell":                  "Message privé",
	"POST /api/servers/:id/inbox/dismiss":         "Modération : élément classé",
	"PUT /api/servers/:id/brand":                  "Couleur et logo du serveur",
}

// audit records the successful dashboard actions in admin_log.
func audit(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		action, ok := auditActions[c.Request.Method+" "+c.FullPath()]
		if !ok {
			return
		}
		c.Next()
		if c.Writer.Status() >= 300 {
			return
		}
		user := c.GetString("user")
		if user == "" {
			user = "local" // no dashboard password
		}
		var server any
		if v, ok := c.Get("audit_server"); ok {
			server = v
		}
		if id, err := strconv.ParseInt(c.Param("id"), 10, 64); err == nil {
			server = id
		}
		detail := c.GetString("audit")
		if n := c.Param("name"); n != "" && detail == "" {
			detail = n
		}
		if _, err := db.Exec(`INSERT INTO admin_log(ts, server_id, username, action, detail) VALUES(?,?,?,?,?)`,
			time.Now().Unix(), server, user, action, query.RedactSecrets(detail)); err != nil {
			slog.Error("admin log", "err", err)
		}
	}
}
