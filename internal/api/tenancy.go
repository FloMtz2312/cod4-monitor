package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// Empty owner denotes the stable .env super-administrator, even after a username change.
const accountSuper = "superadmin"

var permissionNames = map[string]string{
	"private": "Consulter les données privées (IP, chat, logs, notes)",
	"kick":    "Expulser un joueur", "ban": "Bannir et débannir", "reports": "Traiter les signalements",
	"notes": "Modifier les notes joueurs", "maps": "Changer la map / relancer la partie", "say": "Envoyer un message",
	"rcon": "Console RCON complète et commandes programmées (contrôle total du jeu)",
}

func super(c *gin.Context) bool {
	return !c.GetBool("loginmode") || c.GetString("account_role") == accountSuper
}
func owns(db *sql.DB, c *gin.Context, id int64) bool {
	if super(c) {
		return true
	}
	if c.GetString("account_role") != "admin" {
		return false
	}
	var owner string
	return db.QueryRow(`SELECT owner FROM servers WHERE id=?`, id).Scan(&owner) == nil && owner != "" && owner == c.GetString("user")
}
func grants(db *sql.DB, user string, id int64) map[string]bool {
	out := map[string]bool{}
	var raw string
	var names []string
	if db.QueryRow(`SELECT permissions FROM server_members m JOIN users u ON u.username=m.username WHERE m.server_id=? AND m.username=?`, id, user).Scan(&raw) == nil && json.Unmarshal([]byte(raw), &names) == nil {
		for _, p := range names {
			out[p] = true
		}
	}
	return out
}
func permit(db *sql.DB, c *gin.Context, id int64, p string) bool {
	return owns(db, c, id) || grants(db, c.GetString("user"), id)[p]
}
func validPermissions(ps []string) bool {
	for _, p := range ps {
		if _, ok := permissionNames[p]; !ok {
			return false
		}
	}
	return len(ps) <= len(permissionNames)
}
func serverID(c *gin.Context) int64 {
	v := c.Param("id")
	if v == "" {
		v = c.Param("sid")
	}
	id, _ := strconv.ParseInt(v, 10, 64)
	return id
}

// Explicit allowlist: a new endpoint never silently grants a tenant access.
func tenantAllowed(db *sql.DB, c *gin.Context) bool {
	if super(c) {
		return true
	}
	path, method := c.FullPath(), c.Request.Method
	acct := c.GetString("account_role")
	user := c.GetString("user")
	id := serverID(c)
	if id > 0 && owns(db, c, id) {
		c.Set("role", roleAdmin)
	} else if id > 0 && permit(db, c, id, "private") {
		c.Set("role", roleSemi)
	} else {
		c.Set("role", roleVisitor)
	}
	if path == "" {
		return method == "GET" || method == "HEAD"
	}
	if method == "GET" && visitorRoutes[path] {
		return true
	}
	if user == "" {
		return false
	}
	if path == "/api/logout" || path == "/api/me/password" {
		return method == "POST"
	}
	if path == "/api/push" || path == "/api/push/test" { // own browser's notifications; rights checked at each send
		return true
	}
	if path == "/api/invitations" || strings.HasPrefix(path, "/api/invitations/") {
		return acct == "admin"
	}
	if path == "/api/serverconf" || path == "/api/servertest" {
		return acct == "admin"
	}
	if path == "/api/audit" || path == "/api/observability" {
		return acct == "admin" && method == "GET"
	}
	if id <= 0 {
		return false
	}
	if strings.HasPrefix(path, "/api/serverconf/") || strings.Contains(path, "/team") {
		return owns(db, c, id)
	}
	if owns(db, c, id) {
		return true
	}
	suffix := strings.TrimPrefix(path, "/api/servers/:id")
	ps := grants(db, user, id)
	if method == "GET" {
		switch suffix {
		case "/logs", "/chat", "/aliases", "/alts", "/errors", "/pings", "/cvars", "/evasion", "/b3", "/b3/player", "/b3/search", "/b3/evasions", "/b3/stats", "/b3/admins", "/b3/penalties",
			"/demos", "/demos/:did/file/:name":
			return ps["private"]
		case "/notes":
			return ps["private"] || ps["notes"]
		case "/reports", "/inbox":
			return ps["reports"] || ps["private"]
		case "/bans":
			return ps["ban"]
		case "/catalog":
			return ps["maps"] || ps["rcon"]
		case "/dvar", "/rcon/log", "/schedules":
			return ps["rcon"]
		}
		return false
	}
	switch suffix {
	case "/kick":
		return ps["kick"]
	case "/tell":
		return ps["say"]
	case "/ban", "/unban":
		return ps["ban"]
	case "/demos/:did/keep":
		return ps["private"]
	case "/reports/:rid/done", "/inbox/dismiss":
		return ps["reports"]
	case "/notes":
		return ps["notes"]
	case "/rcon":
		return ps["rcon"] || ps["maps"] || ps["say"] // command checked separately before execution
	case "/schedules", "/schedules/:sid/toggle", "/schedules/:sid":
		return ps["rcon"]
	}
	return false
}

func tenancyRoutes(r *gin.Engine, db *sql.DB, adminUser string) {
	acceptanceLimiter := &limiter{fail: map[string][]time.Time{}}
	r.GET("/api/invitations", func(c *gin.Context) {
		q := `SELECT token_hash AS id,username,account_role,server_id,created_at,expires_at,used_at,revoked_at FROM invitations WHERE created_by=? ORDER BY created_at DESC LIMIT 100`
		args := []any{c.GetString("user")}
		if super(c) {
			q = `SELECT token_hash AS id,username,account_role,server_id,created_at,expires_at,used_at,revoked_at FROM invitations ORDER BY created_at DESC LIMIT 100`
			args = nil
		}
		out, err := rows(db, q, args...)
		if err != nil {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"invitations": out, "permissions": permissionNames})
	})
	r.POST("/api/invitations", sameOrigin, func(c *gin.Context) {
		var in struct {
			Username    string   `json:"username"`
			Role        string   `json:"role"`
			ServerID    int64    `json:"server_id"`
			Permissions []string `json:"permissions"`
		}
		if c.ShouldBindJSON(&in) != nil {
			c.Status(400)
			return
		}
		in.Username = strings.ToLower(strings.TrimSpace(in.Username))
		if !usernameRe.MatchString(in.Username) || in.Username == adminUser || !validPermissions(in.Permissions) {
			c.JSON(400, gin.H{"error": "identifiant ou permissions invalides"})
			return
		}
		if in.Role != "admin" && in.Role != "moderator" {
			c.Status(400)
			return
		}
		if in.Role == "admin" {
			if !super(c) {
				c.Status(403)
				return
			}
			in.ServerID = 0
			in.Permissions = nil
		} else {
			var n int
			if in.ServerID <= 0 || db.QueryRow(`SELECT COUNT(*) FROM servers WHERE id=?`, in.ServerID).Scan(&n) != nil || n == 0 || !owns(db, c, in.ServerID) {
				c.Status(403)
				return
			}
		}
		var existing string
		db.QueryRow(`SELECT account_role FROM users WHERE username=?`, in.Username).Scan(&existing)
		if existing != "" && (in.Role == "admin" || existing != "moderator") {
			c.JSON(409, gin.H{"error": "ce compte existe déjà avec un autre rôle"})
			return
		}
		raw := make([]byte, 32)
		if _, err := rand.Read(raw); err != nil {
			c.Status(500)
			return
		}
		token := hex.EncodeToString(raw)
		perms, _ := json.Marshal(in.Permissions)
		now := time.Now().Unix()
		var sid any
		if in.ServerID > 0 {
			sid = in.ServerID
		}
		_, err := db.Exec(`INSERT INTO invitations(token_hash,username,account_role,server_id,permissions,created_by,created_at,expires_at) VALUES(?,?,?,?,?,?,?,?)`, tokenHash(token), in.Username, in.Role, sid, string(perms), c.GetString("user"), now, now+48*3600)
		if err != nil {
			c.Status(500)
			return
		}
		c.Set("audit", in.Username+" · "+in.Role)
		c.JSON(201, gin.H{"token": token, "expires_at": now + 48*3600, "path": "/#/invitation/" + token})
	})
	r.DELETE("/api/invitations/:token", sameOrigin, func(c *gin.Context) {
		q := `UPDATE invitations SET revoked_at=? WHERE token_hash=? AND used_at IS NULL`
		args := []any{time.Now().Unix(), c.Param("token")}
		if !super(c) {
			q += ` AND created_by=?`
			args = append(args, c.GetString("user"))
		}
		res, err := db.Exec(q, args...)
		if err != nil {
			c.Status(500)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.Status(404)
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
	r.POST("/api/invitations/accept", sameOrigin, func(c *gin.Context) {
		if acceptanceLimiter.blocked(clientIP(c)) {
			c.Status(429)
			return
		}
		acceptanceLimiter.failed(clientIP(c))
		var in struct{ Token, Password string }
		if c.ShouldBindJSON(&in) != nil || len(in.Token) != 64 {
			c.Status(400)
			return
		}
		h, err := checkedHash(in.Password)
		if err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		tx, err := db.Begin()
		if err != nil {
			c.Status(500)
			return
		}
		defer tx.Rollback()
		var u, acct, perms, creator string
		var sid sql.NullInt64
		now := time.Now().Unix()
		// Claim first: concurrent accept/revoke cannot both succeed.
		err = tx.QueryRow(`UPDATE invitations SET used_at=? WHERE token_hash=? AND used_at IS NULL AND revoked_at IS NULL AND expires_at>? RETURNING username,account_role,server_id,permissions,created_by`, now, tokenHash(in.Token), now).Scan(&u, &acct, &sid, &perms, &creator)
		if err != nil || u == adminUser {
			c.JSON(400, gin.H{"error": "invitation invalide, expirée ou déjà utilisée"})
			return
		}
		var creatorRole string
		if creator != adminUser {
			tx.QueryRow(`SELECT account_role FROM users WHERE username=?`, creator).Scan(&creatorRole)
			if creatorRole != "admin" {
				c.Status(403)
				return
			}
		}
		if acct == "admin" && creator != adminUser {
			c.Status(403)
			return
		}
		if sid.Valid {
			var owner string
			if tx.QueryRow(`SELECT owner FROM servers WHERE id=?`, sid.Int64).Scan(&owner) != nil || (creator != adminUser && owner != creator) {
				c.JSON(403, gin.H{"error": "le propriétaire du serveur a changé"})
				return
			}
		}
		var oldHash, oldRole string
		err = tx.QueryRow(`SELECT pass_hash,account_role FROM users WHERE username=?`, u).Scan(&oldHash, &oldRole)
		if err == nil {
			if acct != "moderator" || oldRole != "moderator" || !checkPassword(in.Password, oldHash) {
				c.JSON(409, gin.H{"error": "compte existant : utilise son mot de passe actuel"})
				return
			}
		} else if err == sql.ErrNoRows {
			_, err = tx.Exec(`INSERT INTO users(username,pass_hash,created_at,account_role) VALUES(?,?,?,?)`, u, h, now, acct)
			if err != nil {
				c.Status(409)
				return
			}
		} else {
			c.Status(500)
			return
		}
		if sid.Valid {
			_, err = tx.Exec(`INSERT INTO server_members(server_id,username,permissions) VALUES(?,?,?) ON CONFLICT(server_id,username) DO UPDATE SET permissions=excluded.permissions`, sid.Int64, u, perms)
			if err != nil {
				c.Status(500)
				return
			}
		}
		if err = tx.Commit(); err != nil {
			c.Status(500)
			return
		}
		c.Set("audit", u)
		c.JSON(http.StatusOK, gin.H{"ok": true, "username": u})
	})
	r.GET("/api/servers/:id/team", func(c *gin.Context) {
		rs, err := rows(db, `SELECT m.username,m.permissions FROM server_members m JOIN users u ON u.username=m.username WHERE server_id=? ORDER BY m.username`, serverID(c))
		if err != nil {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"members": rs, "permissions": permissionNames})
	})
	r.PUT("/api/servers/:id/team/:name", sameOrigin, func(c *gin.Context) {
		var in struct{ Permissions []string }
		if c.ShouldBindJSON(&in) != nil || !validPermissions(in.Permissions) {
			c.Status(400)
			return
		}
		ps, _ := json.Marshal(in.Permissions)
		tx, err := db.Begin()
		if err != nil {
			c.Status(500)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`UPDATE server_members SET permissions=? WHERE server_id=? AND username=?`, string(ps), serverID(c), c.Param("name"))
		if err != nil {
			c.Status(500)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.Status(404)
			return
		}
		if _, err = tx.Exec(`UPDATE invitations SET revoked_at=? WHERE server_id=? AND username=? AND used_at IS NULL`, time.Now().Unix(), serverID(c), c.Param("name")); err != nil {
			c.Status(500)
			return
		}
		if tx.Commit() != nil {
			c.Status(500)
			return
		}
		c.Set("audit", c.Param("name"))
		c.JSON(200, gin.H{"ok": true})
	})
	r.DELETE("/api/servers/:id/team/:name", sameOrigin, func(c *gin.Context) {
		tx, err := db.Begin()
		if err != nil {
			c.Status(500)
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`DELETE FROM server_members WHERE server_id=? AND username=?`, serverID(c), c.Param("name")); err != nil {
			c.Status(500)
			return
		}
		if _, err = tx.Exec(`UPDATE invitations SET revoked_at=? WHERE server_id=? AND username=? AND used_at IS NULL`, time.Now().Unix(), serverID(c), c.Param("name")); err != nil {
			c.Status(500)
			return
		}
		if tx.Commit() != nil {
			c.Status(500)
			return
		}
		c.Set("audit", c.Param("name"))
		c.JSON(200, gin.H{"ok": true})
	})
	r.PUT("/api/servers/:id/owner", sameOrigin, func(c *gin.Context) {
		if !super(c) {
			c.Status(403)
			return
		}
		var in struct{ Username string }
		if c.ShouldBindJSON(&in) != nil {
			c.Status(400)
			return
		}
		in.Username = strings.ToLower(strings.TrimSpace(in.Username))
		if in.Username == adminUser {
			in.Username = ""
		}
		if in.Username != "" {
			var a string
			if db.QueryRow(`SELECT account_role FROM users WHERE username=?`, in.Username).Scan(&a) != nil || a != "admin" {
				c.JSON(400, gin.H{"error": "choisir un compte administrateur"})
				return
			}
		}
		tx, err := db.Begin()
		if err != nil {
			c.Status(500)
			return
		}
		defer tx.Rollback()
		res, err := tx.Exec(`UPDATE servers SET owner=? WHERE id=?`, in.Username, serverID(c))
		if err != nil {
			c.Status(500)
			return
		}
		if n, _ := res.RowsAffected(); n == 0 {
			c.Status(404)
			return
		}
		// A transfer clears the previous owner's delegated access and outstanding invitations.
		if _, err = tx.Exec(`DELETE FROM server_members WHERE server_id=?`, serverID(c)); err != nil {
			c.Status(500)
			return
		}
		if _, err = tx.Exec(`UPDATE invitations SET revoked_at=? WHERE server_id=? AND used_at IS NULL`, time.Now().Unix(), serverID(c)); err != nil {
			c.Status(500)
			return
		}
		if tx.Commit() != nil {
			c.Status(500)
			return
		}
		c.Set("audit", in.Username)
		c.JSON(200, gin.H{"ok": true})
	})
}

// A stream rechecks both session and membership for every private event.
func eventAllowed(db *sql.DB, c *gin.Context, b []byte) bool {
	if publicEvent(b) {
		return true
	}
	if !c.GetBool("loginmode") {
		return true
	}
	t, err := c.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	var user string
	if db.QueryRow(`SELECT username FROM sessions WHERE token_hash=? AND expires_at>?`, tokenHash(t), time.Now().Unix()).Scan(&user) != nil || user != c.GetString("user") {
		return false
	}
	var ev struct {
		ServerID int64 `json:"server_id"`
	}
	if json.Unmarshal(b, &ev) != nil || ev.ServerID <= 0 {
		return false
	}
	return permit(db, c, ev.ServerID, "private")
}
func permittedCommand(db *sql.DB, c *gin.Context, cmd string) bool {
	id := serverID(c)
	if permit(db, c, id, "rcon") {
		return true
	}
	// Restricted shortcuts cannot inject a second console command.
	if strings.ContainsAny(cmd, "\r\n;\x00\\\"") {
		return false
	}
	words := strings.Fields(cmd)
	if len(words) == 0 {
		return false
	}
	if permit(db, c, id, "maps") {
		if len(words) == 1 && (words[0] == "map_rotate" || words[0] == "fast_restart") {
			return true
		}
		if len(words) == 2 && words[0] == "map" {
			for _, r := range words[1] {
				if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_') {
					return false
				}
			}
			return true
		}
	}
	return permit(db, c, id, "say") && strings.HasPrefix(cmd, "say ") && len(cmd) <= 240
}
