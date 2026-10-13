package api

import (
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// Session authentication; tenancy.go enforces account roles and per-server access.

const (
	sessionCookie = "cod4_session"
	sessionTTL    = 7 * 24 * time.Hour
	pbkdf2Iter    = 600000 // PBKDF2-HMAC-SHA256 work factor
)

var usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_.-]{3,32}$`)

func hashPassword(pw string) (string, error) {
	salt := make([]byte, 16)
	rand.Read(salt)
	key, err := pbkdf2.Key(sha256.New, pw, salt, pbkdf2Iter, 32)
	if err != nil {
		return "", err
	}
	b := base64.RawStdEncoding
	return "pbkdf2-sha256$" + strconv.Itoa(pbkdf2Iter) + "$" + b.EncodeToString(salt) + "$" + b.EncodeToString(key), nil
}

func checkPassword(pw, stored string) bool {
	f := strings.Split(stored, "$")
	if len(f) != 4 || f[0] != "pbkdf2-sha256" {
		return false
	}
	iter, err1 := strconv.Atoi(f[1])
	salt, err2 := base64.RawStdEncoding.DecodeString(f[2])
	want, err3 := base64.RawStdEncoding.DecodeString(f[3])
	if err1 != nil || err2 != nil || err3 != nil || iter < 100000 || iter > 2000000 || len(salt) != 16 || len(want) != 32 {
		return false
	}
	key, err := pbkdf2.Key(sha256.New, pw, salt, iter, len(want))
	return err == nil && subtle.ConstantTimeCompare(key, want) == 1
}

func tokenHash(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// limiter: at most `max` failed logins per client IP in `window`.
type limiter struct {
	mu   sync.Mutex
	fail map[string][]time.Time
}

const loginMaxFails, loginWindow = 10, 15 * time.Minute

func (l *limiter) blocked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, times := range l.fail {
		if len(times) == 0 || time.Since(times[len(times)-1]) >= loginWindow {
			delete(l.fail, key)
		}
	}
	if _, ok := l.fail[ip]; !ok && len(l.fail) >= 10000 {
		return true
	}
	recent := l.fail[ip][:0]
	for _, t := range l.fail[ip] {
		if time.Since(t) < loginWindow {
			recent = append(recent, t)
		}
	}
	l.fail[ip] = recent
	return len(recent) >= loginMaxFails
}

func (l *limiter) failed(ip string) {
	l.mu.Lock()
	l.fail[ip] = append(l.fail[ip], time.Now())
	l.mu.Unlock()
}

// Proxy middleware supplies these only after validating the direct peer.
func clientIP(c *gin.Context) string { return c.ClientIP() }
func secureRequest(c *gin.Context) bool {
	return c.Request.TLS != nil || c.GetBool("trusted_https")
}

const (
	roleAdmin   = "admin"
	roleSemi    = "semi"
	roleVisitor = "visitor"
)

// role of the request; without a dashboard password (local use) everyone is admin.
func role(c *gin.Context) string {
	if r := c.GetString("role"); r != "" {
		return r
	}
	return roleAdmin
}

// visitorRoutes: what a visitor (no account) may read, by gin route pattern.
var visitorRoutes = map[string]bool{
	"/api/servers/:id/form": true, "/api/servers/:id/challenges": true, "/api/servers/:id/card.png": true,
	"/api/me": true, "/api/servers": true, "/api/events": true, "/api/servers/:id/rcon": true,
	"/api/servers/:id/status": true, "/api/servers/:id/history": true, "/api/servers/:id/uptime": true,
	"/api/servers/:id/players": true, "/api/servers/:id/player": true, "/api/servers/:id/maps": true,
	"/api/servers/:id/records": true, "/api/servers/:id/combat": true, "/api/servers/:id/rounds": true,
	"/api/servers/:id/heatmap": true, "/api/servers/:id/duel": true, "/api/servers/:id/matches": true,
	"/api/servers/:id/match": true, "/api/servers/:id/badges": true, "/api/servers/:id/progress": true,
	"/api/servers/:id/mapstats": true, "/api/servers/:id/mapvotes": true, "/api/servers/:id/maprank": true, "/api/servers/:id/week": true,
	"/api/servers/:id/maprounds": true, "/api/servers/:id/retention": true, "/api/servers/:id/live": true, "/api/servers/:id/feed": true,
	"/api/claim/:code": true, "/me/:code": true, "/api/servers/:id/logo.png": true,
}

// public paths: signing in, accepting an invitation, the Docker healthcheck. The interface itself (its page and
// static files) is public too: it only shows what /api/me allows.
func publicPath(p string) bool {
	switch p {
	case "/api/invitations/accept", "/api/health", "/api/login":
		return true
	}
	return false
}

// authRoutes installs the login middleware (protects every route registered afterwards) and the account routes.
func authRoutes(r *gin.Engine, db *sql.DB, adminUser, adminPass string) {
	lim := &limiter{fail: map[string][]time.Time{}}
	adminUser = strings.ToLower(strings.TrimSpace(adminUser))
	if adminUser == "" {
		panic("DASHBOARD_USER is required")
	}
	// Changing the .env administrator credentials invalidates all existing admin sessions.
	credential := adminUser + "\x00" + adminPass
	var old string
	db.QueryRow(`SELECT value FROM meta WHERE key='admin_credentials'`).Scan(&old)
	if !checkPassword(credential, old) {
		fingerprint, err := hashPassword(credential)
		if err != nil {
			panic(err)
		}
		if _, err := db.Exec(`DELETE FROM sessions WHERE username=? OR username NOT IN (SELECT username FROM users)`, adminUser); err != nil {
			panic(err)
		}
		if _, err := db.Exec(`INSERT INTO meta(key,value) VALUES('admin_credentials',?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, fingerprint); err != nil {
			panic(err)
		}
	}

	userOf := func(c *gin.Context) string {
		t, err := c.Cookie(sessionCookie)
		if err != nil || t == "" {
			return ""
		}
		var u string
		if len(t) != 64 || db.QueryRow(`SELECT username FROM sessions WHERE token_hash=? AND expires_at>?`, tokenHash(t), time.Now().Unix()).Scan(&u) != nil {
			return ""
		}
		if u != adminUser { // account deleted meanwhile
			var n int
			if db.QueryRow(`SELECT COUNT(*) FROM users WHERE username=?`, u).Scan(&n); n == 0 {
				return ""
			}
		}
		return u
	}

	// Preserve legacy read-only access on existing servers, exactly once.
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	var migrated int
	if err = tx.QueryRow(`SELECT COUNT(*) FROM meta WHERE key='tenant_migration_v1'`).Scan(&migrated); err != nil {
		tx.Rollback()
		panic(err)
	}
	if migrated == 0 {
		if _, err = tx.Exec(`INSERT OR IGNORE INTO server_members(server_id,username,permissions) SELECT s.id,u.username,'["private"]' FROM servers s CROSS JOIN users u`); err != nil {
			tx.Rollback()
			panic(err)
		}
		if _, err = tx.Exec(`INSERT INTO meta(key,value) VALUES('tenant_migration_v1','1')`); err != nil {
			tx.Rollback()
			panic(err)
		}
	}
	if err = tx.Commit(); err != nil {
		panic(err)
	}
	var middleware gin.HandlerFunc = func(c *gin.Context) {
		c.Set("loginmode", true)
		if publicPath(c.Request.URL.Path) {
			return
		}
		u, account := userOf(c), roleVisitor
		if u == adminUser {
			account = accountSuper
		} else if u != "" {
			if db.QueryRow(`SELECT account_role FROM users WHERE username=?`, u).Scan(&account) != nil {
				u = ""
				account = roleVisitor
			}
		}
		c.Set("user", u)
		c.Set("account_role", account)
		c.Set("role", roleVisitor)
		if account == accountSuper {
			c.Set("role", roleAdmin)
		}
		if !tenantAllowed(db, c) {
			status := 403
			if u == "" {
				status = 401
			}
			c.AbortWithStatusJSON(status, gin.H{"error": "accès non autorisé pour ce serveur ou cette action"})
		}
	}
	r.Use(middleware, audit(db)) // before the routes below: gin only applies middleware to routes registered after it
	adminOnly := func(c *gin.Context) {
		if c.GetString("user") != adminUser {
			c.AbortWithStatusJSON(403, gin.H{"error": "réservé à l'administrateur"})
		}
	}

	tenancyRoutes(r, db, adminUser)
	r.POST("/api/login", sameOrigin, func(c *gin.Context) {
		ip := clientIP(c)
		if lim.blocked(ip) {
			c.JSON(429, gin.H{"error": "trop de tentatives, réessaie dans 15 minutes"})
			return
		}
		var req struct{ Username, Password string }
		if c.ShouldBindJSON(&req) != nil || len(req.Username) > 128 || len(req.Password) > 128 {
			c.JSON(400, gin.H{"error": "identifiants invalides"})
			return
		}
		u := strings.ToLower(strings.TrimSpace(req.Username))
		ok := false
		if u == adminUser {
			ok = subtle.ConstantTimeCompare([]byte(tokenHash(req.Password)), []byte(tokenHash(adminPass))) == 1
		} else {
			var h string
			if db.QueryRow(`SELECT pass_hash FROM users WHERE username=?`, u).Scan(&h) == nil {
				ok = checkPassword(req.Password, h)
			} else {
				checkPassword(req.Password, "pbkdf2-sha256$"+strconv.Itoa(pbkdf2Iter)+"$AAAAAAAAAAAAAAAAAAAAAA$AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA") // same cost: no user enumeration by timing
			}
		}
		if !ok {
			lim.failed(ip)
			time.Sleep(500 * time.Millisecond)
			c.JSON(401, gin.H{"error": "identifiant ou mot de passe incorrect"})
			return
		}
		raw := make([]byte, 32)
		rand.Read(raw)
		token := hex.EncodeToString(raw)
		now := time.Now()
		db.Exec(`DELETE FROM sessions WHERE expires_at<?`, now.Unix())
		if _, err := db.Exec(`INSERT INTO sessions VALUES(?,?,?,?)`, tokenHash(token), u, now.Unix(), now.Add(sessionTTL).Unix()); err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		db.Exec(`DELETE FROM sessions WHERE username=? AND token_hash NOT IN (SELECT token_hash FROM sessions WHERE username=? ORDER BY created_at DESC,rowid DESC LIMIT 10)`, u, u)
		db.Exec(`UPDATE users SET last_login=? WHERE username=?`, now.Unix(), u)
		c.SetSameSite(3) // http.SameSiteStrictMode
		c.SetCookie(sessionCookie, token, int(sessionTTL.Seconds()), "/", "", secureRequest(c), true)
		c.JSON(200, gin.H{"ok": true})
	})

	r.POST("/api/logout", sameOrigin, func(c *gin.Context) {
		if t, err := c.Cookie(sessionCookie); err == nil {
			db.Exec(`DELETE FROM sessions WHERE token_hash=?`, tokenHash(t))
		}
		c.SetCookie(sessionCookie, "", -1, "/", "", secureRequest(c), true)
		c.JSON(200, gin.H{"ok": true})
	})

	r.GET("/api/me", func(c *gin.Context) {
		access := map[int64]any{}
		rs, err := rows(db, `SELECT id FROM servers`)
		if err != nil {
			c.Status(500)
			return
		}
		for _, row := range rs {
			id := row["id"].(int64)
			ps := grants(db, c.GetString("user"), id)
			owner := owns(db, c, id)
			access[id] = gin.H{"owner": owner, "permissions": ps}
		}
		c.JSON(200, gin.H{"username": c.GetString("user"), "role": c.GetString("account_role"), "admin": super(c), "access": access})
	})

	// own password (not for the .env account: change it in the .env)
	r.POST("/api/me/password", sameOrigin, func(c *gin.Context) {
		u := c.GetString("user")
		var req struct{ Current, New string }
		c.ShouldBindJSON(&req)
		var h string
		if u == adminUser {
			c.JSON(400, gin.H{"error": "le mot de passe du compte principal se change dans le .env (DASHBOARD_PASSWORD)"})
			return
		}
		if db.QueryRow(`SELECT pass_hash FROM users WHERE username=?`, u).Scan(&h) != nil || !checkPassword(req.Current, h) {
			c.JSON(400, gin.H{"error": "mot de passe actuel incorrect"})
			return
		}
		if err := setPassword(db, u, req.New); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})

	a := r.Group("/api/users", adminOnly)
	a.GET("", func(c *gin.Context) {
		rs, err := rows(db, `SELECT username, account_role, created_at, last_login FROM users ORDER BY username`)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"users": rs, "admin": adminUser})
	})
	a.POST("", sameOrigin, func(c *gin.Context) { c.JSON(410, gin.H{"error": "utiliser une invitation pour créer un compte"}) })
	a.POST("/:name/password", sameOrigin, func(c *gin.Context) {
		var req struct{ Password string }
		c.ShouldBindJSON(&req)
		if err := setPassword(db, c.Param("name"), req.Password); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		db.Exec(`DELETE FROM sessions WHERE username=?`, c.Param("name")) // log them out everywhere
		c.JSON(200, gin.H{"ok": true})
	})
	a.DELETE("/:name", sameOrigin, func(c *gin.Context) {
		u := c.Param("name")
		var n int
		if db.QueryRow(`SELECT COUNT(*) FROM servers WHERE owner=?`, u).Scan(&n) != nil {
			c.Status(500)
			return
		}
		if n > 0 {
			c.JSON(409, gin.H{"error": "transférer les serveurs avant de supprimer leur propriétaire"})
			return
		}
		tx, err := db.Begin()
		if err != nil {
			c.Status(500)
			return
		}
		defer tx.Rollback()
		for _, q := range []string{`DELETE FROM users WHERE username=?`, `DELETE FROM sessions WHERE username=?`, `DELETE FROM server_members WHERE username=?`, `DELETE FROM invitations WHERE username=? OR created_by=?`} {
			args := []any{u}
			if strings.Contains(q, "OR created_by") {
				args = append(args, u)
			}
			if _, err = tx.Exec(q, args...); err != nil {
				c.Status(500)
				return
			}
		}
		if tx.Commit() != nil {
			c.Status(500)
			return
		}
		c.JSON(200, gin.H{"ok": true})
	})
}

func checkedHash(pw string) (string, error) {
	if len(pw) < 8 || len(pw) > 128 {
		return "", errors.New("le mot de passe doit faire au moins 8 caractères")
	}
	return hashPassword(pw)
}

func setPassword(db *sql.DB, user, pw string) error {
	h, err := checkedHash(pw)
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`UPDATE users SET pass_hash=? WHERE username=?`, h, user)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return errors.New("compte inconnu")
	}
	if _, err := tx.Exec(`DELETE FROM sessions WHERE username=?`, user); err != nil {
		return err
	}
	return tx.Commit()
}
