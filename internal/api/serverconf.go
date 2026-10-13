package api

import (
	"context"
	"errors"
	"net"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"cod4monitor/internal/b3"
	"cod4monitor/internal/logtail"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// RconOf returns the rcon of a server, nil when it has none.
type RconOf func(id int64) func(cmd string) (string, error)

func (r RconOf) of(id int64) func(string) (string, error) {
	if r == nil {
		return nil
	}
	return r(id)
}

// ServerManager starts, restarts or stops the monitoring of a server after its settings changed.
type ServerManager interface {
	Apply(id int64) error
}

// serverForm is what the dashboard sends. Empty passwords keep the saved ones (they are never sent back).
type serverForm struct {
	Name     string `json:"name"`
	Address  string `json:"address"` // host:port of the game server
	FTPTLS   bool   `json:"ftp_tls"`
	FTPHost  string `json:"ftp_host"`
	FTPPort  int    `json:"ftp_port"`
	FTPUser  string `json:"ftp_user"`
	FTPPass  string `json:"ftp_pass"`
	FTPPath  string `json:"ftp_path"` // folder of the logs: /mods/<mod>/
	FTPClear bool   `json:"ftp_clear"`
	LogDir   string `json:"log_dir"` // instead of FTP: folder on this machine, inside LOCAL_LOGS_ROOT
	// advanced: log names or paths, time zone of the server's clock (empty = default)
	GamesLog      string       `json:"games_log"`
	ConsoleLog    string       `json:"console_log"`
	EnterLeaveLog string       `json:"enterleave_log"`
	LogTZ         string       `json:"log_tz"`
	B3DB          string       `json:"b3_db"`    // B3's SQLite database, inside B3_ROOT (super administrator)
	B3MySQL       *b3MySQLForm `json:"b3_mysql"` // or B3's MySQL / MariaDB database (any owner)
	B3Clear       bool         `json:"b3_clear"`
	Rcon          string       `json:"rcon"`
	RconOff       bool         `json:"rcon_clear"`
	Enabled       *bool        `json:"enabled"`
}

// b3MySQLForm: the connection to a B3 kept in MySQL / MariaDB. An empty password keeps the saved one.
type b3MySQLForm struct {
	Host string `json:"host"` // or the path of MySQL's socket file, inside B3_ROOT (a MariaDB next to B3)
	Port int    `json:"port"`
	User string `json:"user"`
	Pass string `json:"pass"`
	Name string `json:"name"` // database
	TLS  bool   `json:"tls"`
}

// b3Local: the form (or the saved settings) use a B3 file on this machine (SQLite, or MySQL's socket), which only the
// super administrator may set.
func (f serverForm) b3Local(prev store.ServerConf) bool {
	if strings.TrimSpace(f.B3DB) != "" || f.B3MySQL != nil && strings.HasPrefix(strings.TrimSpace(f.B3MySQL.Host), "/") && !f.B3Clear {
		return true
	}
	saved := prev.Logs.B3DB != "" && (!b3.IsMySQL(prev.Logs.B3DB) || strings.Contains(prev.Logs.B3DB, "socket="))
	return saved && !f.B3Clear && f.B3MySQL == nil
}

// publicHost refuses an address of this machine or of its local network: an owner who is not the super
// administrator must not make the monitor connect to services it alone can reach.
func publicHost(host string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(ips) == 0 {
		return errors.New("hôte MySQL introuvable : " + host)
	}
	for _, ip := range ips {
		if a := ip.IP; a.IsLoopback() || a.IsPrivate() || a.IsUnspecified() || a.IsLinkLocalUnicast() || a.IsLinkLocalMulticast() || a.IsMulticast() {
			return errors.New("hôte MySQL refusé : une adresse locale ou privée (seul le super administrateur peut l'utiliser)")
		}
	}
	return nil
}

// conf checks the form and merges it into prev (the saved settings, zero for a new server).
func (f serverForm) conf(prev store.ServerConf) (store.ServerConf, error) {
	c := prev
	c.Name = strings.TrimSpace(f.Name)
	if c.Name == "" {
		return c, errors.New("donne un nom au serveur")
	}
	host, portS, err := net.SplitHostPort(strings.TrimSpace(f.Address))
	port, perr := strconv.Atoi(portS)
	if err != nil || perr != nil || host == "" || port < 1 || port > 65535 {
		return c, errors.New("adresse du serveur invalide, attendu ip:port (ex. 203.0.113.10:28960)")
	}
	c.Host, c.Port = host, port
	if f.Enabled != nil {
		c.Enabled = *f.Enabled
	} else if prev.ID == 0 {
		c.Enabled = true
	}

	switch {
	case f.RconOff:
		c.RconPass = ""
	case f.Rcon != "":
		if strings.ContainsAny(f.Rcon, "\" \n\r") {
			return c, errors.New("le mot de passe rcon ne peut pas contenir d'espace ni de guillemet")
		}
		c.RconPass = f.Rcon
	}

	local := strings.TrimSpace(f.LogDir) != "" && !f.FTPClear
	prevB3 := c.Logs.B3DB
	if err := f.advanced(&c, local); err != nil {
		return c, err
	}
	if err := f.b3(&c, prevB3); err != nil {
		return c, err
	}
	if local {
		p := strings.TrimSuffix(path.Clean("/"+strings.Trim(strings.TrimSpace(f.LogDir), "/")), "/") + "/"
		if strings.HasSuffix(p, ".log/") { // the path of games_mp.log itself
			p = path.Dir(strings.TrimSuffix(p, "/")) + "/"
		}
		if !logtail.LocalOK(p) {
			return c, errors.New("dossier local refusé : il doit être dans " + logtail.LocalRoot + " (LOCAL_LOGS_ROOT)")
		}
		c.LogURL = (&url.URL{Scheme: "file", Path: p}).String()
		return c, nil
	}
	if f.FTPClear || strings.TrimSpace(f.FTPHost) == "" {
		c.LogURL = ""
		return c, nil
	}
	pass := f.FTPPass
	if pass == "" && prev.LogURL != "" { // keep the saved password
		if u, err := url.Parse(prev.LogURL); err == nil && u.User != nil {
			pass, _ = u.User.Password()
		}
	}
	ftpPort := f.FTPPort
	if ftpPort == 0 {
		ftpPort = 21
	}
	if ftpPort < 1 || ftpPort > 65535 {
		return c, errors.New("port FTP invalide")
	}
	p := "/" + strings.Trim(strings.TrimSpace(f.FTPPath), "/") + "/"
	if strings.HasSuffix(p, ".log/") { // the path of games_mp.log itself
		p = path.Dir(strings.TrimSuffix(p, "/")) + "/"
	}
	if p == "//" {
		return c, errors.New("indique le dossier des logs, celui du mod (ex. /mods/mon_mod/)")
	}
	u := &url.URL{Scheme: "ftp", Host: net.JoinHostPort(strings.TrimSpace(f.FTPHost), strconv.Itoa(ftpPort)), Path: p}
	if user := strings.TrimSpace(f.FTPUser); user != "" {
		u.User = url.UserPassword(user, pass)
	}
	if f.FTPTLS {
		u.Scheme = "ftps"
	}
	c.LogURL = u.String()
	return c, nil
}

// advanced checks the advanced settings and copies them into c.
func (f serverForm) advanced(c *store.ServerConf, local bool) error {
	c.Logs = store.LogFiles{}
	for _, x := range []struct {
		in  string
		out *string
	}{{f.GamesLog, &c.Logs.Games}, {f.ConsoleLog, &c.Logs.Console}, {f.EnterLeaveLog, &c.Logs.EnterLeave}} {
		v := strings.TrimSpace(x.in)
		if v == "" {
			continue
		}
		if strings.Contains(v, "..") || strings.ContainsAny(v, "\x00\r\n") {
			return errors.New("chemin de log invalide : " + v)
		}
		if local && path.IsAbs(v) && !logtail.LocalOK(v) {
			return errors.New("chemin de log refusé : il doit être dans " + logtail.LocalRoot + " (LOCAL_LOGS_ROOT)")
		}
		*x.out = v
	}
	if p := strings.TrimSpace(f.B3DB); p != "" {
		if !b3.PathOK(p) {
			return errors.New("base B3 refusée : un fichier .db dans " + b3.Root + " (B3_ROOT), ex. " + b3.Root + "b3.db")
		}
		c.Logs.B3DB = path.Clean(p)
	}
	if tz := strings.TrimSpace(f.LogTZ); tz != "" {
		if _, err := time.LoadLocation(tz); err != nil {
			return errors.New("fuseau horaire inconnu : " + tz + " (ex. Europe/Paris, UTC)")
		}
		c.Logs.TZ = tz
	}
	return nil
}

// b3 sets the MySQL connection of B3 from the form (the SQLite file is set by advanced).
func (f serverForm) b3(c *store.ServerConf, prev string) error {
	m := f.B3MySQL
	if f.B3Clear || c.Logs.B3DB != "" || m == nil || strings.TrimSpace(m.Host) == "" {
		if f.B3Clear {
			c.Logs.B3DB = ""
		}
		return nil
	}
	host, user, name := strings.TrimSpace(m.Host), strings.TrimSpace(m.User), strings.TrimSpace(m.Name)
	port := m.Port
	if port == 0 {
		port = 3306
	}
	socket := ""
	if strings.HasPrefix(host, "/") { // MySQL's socket file
		socket, host, port = path.Clean(host), "localhost", 3306
		if !b3.SocketOK(socket) {
			return b3.ErrSocket
		}
	}
	if port < 1 || port > 65535 || user == "" || name == "" || strings.ContainsAny(host+user+name, "/@?#` \x00") {
		return errors.New("connexion MySQL de B3 incomplète : hôte, port, utilisateur et base")
	}
	pass := m.Pass
	if pass == "" && b3.IsMySQL(prev) { // keep the saved password
		if u, err := url.Parse(prev); err == nil && u.User != nil {
			pass, _ = u.User.Password()
		}
	}
	u := &url.URL{Scheme: "mysql", User: url.UserPassword(user, pass), Host: net.JoinHostPort(host, strconv.Itoa(port)), Path: "/" + name}
	if pass == "" {
		u.User = url.User(user)
	}
	q := url.Values{}
	if m.TLS && socket == "" {
		q.Set("tls", "true")
	}
	if socket != "" {
		q.Set("socket", socket)
	}
	u.RawQuery = q.Encode()
	if _, err := b3.MySQLConfig(u.String()); err != nil {
		return err
	}
	c.Logs.B3DB = u.String()
	return nil
}

// serverView: a server's settings for the admin, without its passwords.
func serverView(c store.ServerConf) gin.H {
	v := gin.H{"id": c.ID, "owner": c.Owner, "name": c.Name, "address": net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		"enabled": c.Enabled, "managed": c.Managed, "has_rcon": c.RconPass != "", "ftp": nil, "log_dir": nil,
		"games_log": c.Logs.Games, "console_log": c.Logs.Console, "enterleave_log": c.Logs.EnterLeave, "log_tz": c.Logs.TZ, "b3_db": c.Logs.B3DB,
		"b3_mysql": nil}
	if b3.IsMySQL(c.Logs.B3DB) { // never the password
		v["b3_db"] = ""
		if m, err := b3.MySQLConfig(c.Logs.B3DB); err == nil {
			host, port, _ := net.SplitHostPort(m.Addr)
			if m.Net == "unix" {
				host, port = m.Addr, "3306" // unused with a socket
			}
			p, _ := strconv.Atoi(port)
			v["b3_mysql"] = gin.H{"host": host, "port": p, "user": m.User, "name": m.DBName, "tls": m.TLSConfig != "", "has_pass": m.Passwd != ""}
		}
	}
	if u, err := url.Parse(c.LogURL); c.LogURL != "" && err == nil && logtail.IsLocal(u) {
		v["log_dir"] = u.Path
	} else if c.LogURL != "" && err == nil {
		_, hasPass := u.User.Password()
		port, _ := strconv.Atoi(u.Port())
		if port == 0 {
			port = 21
		}
		v["ftp"] = gin.H{"tls": u.Scheme == "ftps", "host": u.Hostname(), "port": port, "user": u.User.Username(), "path": u.Path, "has_pass": hasPass}
	}
	return v
}

// serverConfRoutes: the admin adds, edits, tests and turns on / off the monitored servers.
func serverConfRoutes(r *gin.Engine, st *store.Store, m ServerManager) {
	fail := func(c *gin.Context, code int, err error) bool {
		if err != nil {
			c.JSON(code, gin.H{"error": err.Error()})
		}
		return err != nil
	}
	apply := func(c *gin.Context, id int64) {
		if m == nil {
			return
		}
		if err := m.Apply(id); err != nil {
			c.JSON(200, gin.H{"id": id, "warning": "enregistré, mais la surveillance n'a pas démarré : " + err.Error()})
			c.Abort()
		}
	}

	r.GET("/api/serverconf", func(c *gin.Context) {
		cs, err := st.ServerConfs()
		if fail(c, 500, err) {
			return
		}
		out := []gin.H{}
		for _, x := range cs {
			if !owns(st.DB, c, x.ID) {
				continue
			}
			out = append(out, serverView(x))
		}
		c.JSON(200, gin.H{"servers": out, "local_root": logtail.LocalRoot, "b3_root": b3.Root})
	})

	save := func(c *gin.Context, id int64) {
		var f serverForm
		if fail(c, 400, c.ShouldBindJSON(&f)) {
			return
		}
		var prev store.ServerConf
		if id != 0 {
			p, ok, err := st.ServerConf(id)
			if fail(c, 500, err) {
				return
			}
			if !ok {
				c.JSON(404, gin.H{"error": "serveur introuvable"})
				return
			}
			prev = p
		}
		if !super(c) && (f.LogDir != "" || f.b3Local(prev) || strings.HasPrefix(prev.LogURL, "file:")) {
			c.JSON(403, gin.H{"error": "les fichiers locaux (logs, base B3 SQLite) sont réservés au super administrateur"})
			return
		}
		if m := f.B3MySQL; !super(c) && m != nil && strings.TrimSpace(m.Host) != "" && !strings.HasPrefix(strings.TrimSpace(m.Host), "/") && !f.B3Clear {
			if fail(c, 400, publicHost(strings.TrimSpace(m.Host))) {
				return
			}
		}
		conf, err := f.conf(prev)
		if fail(c, 400, err) {
			return
		}
		if id == 0 && !super(c) {
			conf.Owner = c.GetString("user")
		}
		id, err = st.SaveServerConf(conf)
		if errors.Is(err, store.ErrServerExists) {
			fail(c, 409, err)
			return
		}
		if fail(c, 500, err) {
			return
		}
		c.Set("audit", conf.Name+" ("+net.JoinHostPort(conf.Host, strconv.Itoa(conf.Port))+")"+
			map[bool]string{true: " · logs FTP", false: ""}[conf.LogURL != ""]+map[bool]string{true: " · rcon", false: ""}[conf.RconPass != ""])
		c.Set("audit_server", id)
		apply(c, id)
		if !c.IsAborted() {
			c.JSON(200, gin.H{"id": id})
		}
	}
	r.POST("/api/serverconf", sameOrigin, func(c *gin.Context) { save(c, 0) })
	r.PUT("/api/serverconf/:sid", sameOrigin, func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("sid"), 10, 64)
		if fail(c, 400, err) {
			return
		}
		save(c, id)
	})

	r.POST("/api/serverconf/:sid/enabled", sameOrigin, func(c *gin.Context) {
		id, err := strconv.ParseInt(c.Param("sid"), 10, 64)
		if fail(c, 400, err) {
			return
		}
		var b struct{ Enabled bool }
		if fail(c, 400, c.ShouldBindJSON(&b)) || fail(c, 500, st.SetServerEnabled(id, b.Enabled)) {
			return
		}
		if conf, ok, _ := st.ServerConf(id); ok {
			c.Set("audit", conf.Name+map[bool]string{true: " : surveillance activée", false: " : surveillance arrêtée"}[b.Enabled])
		}
		c.Set("audit_server", id)
		apply(c, id)
		if !c.IsAborted() {
			c.JSON(200, gin.H{"ok": true})
		}
	})

	// Checks the settings before saving: the game server answers, the rcon password works, the FTP folder holds the logs.
	r.POST("/api/servertest", sameOrigin, func(c *gin.Context) {
		var f struct {
			serverForm
			ID int64 `json:"id"` // editing: empty passwords are taken from the saved settings
		}
		if fail(c, 400, c.ShouldBindJSON(&f)) {
			return
		}
		var prev store.ServerConf
		if f.ID != 0 {
			if !owns(st.DB, c, f.ID) {
				c.Status(403)
				return
			}
			prev, _, _ = st.ServerConf(f.ID)
		}
		if strings.TrimSpace(f.Name) == "" {
			f.Name = "test"
		}
		if !super(c) && (f.LogDir != "" || f.b3Local(prev) || strings.HasPrefix(prev.LogURL, "file:")) {
			c.JSON(403, gin.H{"error": "les fichiers locaux (logs, base B3 SQLite) sont réservés au super administrateur"})
			return
		}
		if m := f.B3MySQL; !super(c) && m != nil && strings.TrimSpace(m.Host) != "" && !strings.HasPrefix(strings.TrimSpace(m.Host), "/") && !f.B3Clear {
			if fail(c, 400, publicHost(strings.TrimSpace(m.Host))) {
				return
			}
		}
		conf, err := f.conf(prev)
		if fail(c, 400, err) {
			return
		}
		addr := net.JoinHostPort(conf.Host, strconv.Itoa(conf.Port))
		out := gin.H{}
		if s, err := query.Query(addr, 3*time.Second); err != nil {
			out["query"] = gin.H{"ok": false, "error": "le serveur ne répond pas : " + err.Error()}
		} else {
			out["query"] = gin.H{"ok": true, "hostname": s.Hostname, "map": s.Map, "players": len(s.Players), "max": s.MaxPlayers}
		}
		if conf.RconPass != "" {
			r, err := query.Rcon(addr, conf.RconPass, "status", 3*time.Second)
			low := strings.ToLower(r)
			switch {
			case err != nil:
				out["rcon"] = gin.H{"ok": false, "error": "pas de réponse : " + err.Error()}
			case strings.Contains(low, "invalid password") || strings.Contains(low, "bad rcon"):
				out["rcon"] = gin.H{"ok": false, "error": "mot de passe rcon refusé"}
			default:
				out["rcon"] = gin.H{"ok": true}
			}
		}
		if conf.LogURL != "" {
			u, _ := url.Parse(conf.LogURL)
			ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
			p, err := logtail.ProbeDir(ctx, u, conf.Logs)
			cancel()
			switch {
			case err != nil:
				out["ftp"] = gin.H{"ok": false, "error": err.Error(), "mods": p.Mods}
			case len(p.Found) == 0:
				out["ftp"] = gin.H{"ok": false, "error": "connecté, mais aucun des logs attendus dans " + u.Path + " (voir les réglages avancés pour d'autres noms)", "mods": p.Mods}
			default:
				out["ftp"] = gin.H{"ok": true, "found": p.Found}
			}
		}
		if conf.Logs.B3DB != "" {
			if d, err := b3.Open(conf.Logs.B3DB); err != nil {
				out["b3"] = gin.H{"ok": false, "error": err.Error()}
			} else {
				admins, _ := d.Admins()
				var n int64
				d.QueryRow(`SELECT COUNT(*) FROM penalties`).Scan(&n)
				d.Close()
				out["b3"] = gin.H{"ok": true, "admins": len(admins), "penalties": n}
			}
		}
		c.JSON(200, out)
	})
}
