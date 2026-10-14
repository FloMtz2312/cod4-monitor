// Package b3 reads and writes the database of a B3 (Big Brother Bot) admin bot: penalties, admins and groups. B3
// stores them in SQLite (a b3.db file on this machine) or in MySQL / MariaDB (B3's storage: mysql://… setting);
// the queries here run on both.
//
// B3 keeps running while the dashboard writes into its database:
//   - a ban or temp ban written here is enforced by B3 itself: every 10 s it kicks the banned players still online,
//     and it checks the bans of every player who connects;
//   - a new group applies when the player connects next: B3 reads the group from its database when a player joins.
package b3

import (
	"database/sql"
	"errors"
	"net"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// Root is the only folder a B3 database can be opened from (B3_ROOT). With Docker, docker-compose.b3.yml mounts
// B3's data folder there.
var Root = "/b3/"

// SetRoot changes Root ("" keeps the default).
func SetRoot(p string) {
	if p = strings.TrimSpace(p); p != "" {
		Root = strings.TrimSuffix(path.Clean(p), "/") + "/"
	}
}

// SocketOK tells whether a MySQL socket file is allowed: an absolute path inside Root.
func SocketOK(p string) bool {
	c := path.Clean(p)
	return path.IsAbs(p) && strings.HasPrefix(c, Root) && !strings.ContainsAny(p, "\x00?#")
}

// PathOK tells whether a database path is allowed: an absolute .db file inside Root.
func PathOK(p string) bool {
	c := path.Clean(p)
	return path.IsAbs(p) && strings.HasSuffix(c, ".db") && strings.HasPrefix(c, Root)
}

var (
	ErrPath     = errors.New("base B3 refusée : un fichier .db dans le dossier B3_ROOT")
	ErrMySQL    = errors.New("adresse MySQL de B3 invalide : mysql://utilisateur:motdepasse@hôte:3306/base")
	ErrSocket   = errors.New("socket MySQL refusé : un fichier dans le dossier B3_ROOT")
	ErrNoClient = errors.New("joueur inconnu de B3 : il doit s'être connecté au serveur depuis l'installation de B3")
	ErrNoGroup  = errors.New("grade B3 inconnu")
)

type DB struct {
	*sql.DB
	my bool // MySQL / MariaDB, else SQLite
}

// IsMySQL: the source is a MySQL / MariaDB address (mysql://user:password@host:3306/database), not a b3.db file.
func IsMySQL(src string) bool { return strings.HasPrefix(src, "mysql://") }

// MySQLConfig reads mysql://user:password@host[:3306]/database[?tls=true|skip-verify], or
// mysql://user:password@localhost/database?socket=/b3/mysqld/mysqld.sock: MySQL's socket file, inside Root (a MariaDB
// next to B3 on this machine, reached without opening a port).
func MySQLConfig(src string) (*mysql.Config, error) {
	u, err := url.Parse(src)
	if err != nil || u.Scheme != "mysql" || u.Hostname() == "" || u.User == nil || u.User.Username() == "" {
		return nil, ErrMySQL
	}
	socket := u.Query().Get("socket")
	if socket != "" && !SocketOK(socket) {
		return nil, ErrSocket
	}
	name := strings.Trim(u.Path, "/")
	if name == "" || strings.ContainsAny(name, "/`") {
		return nil, ErrMySQL
	}
	port := u.Port()
	if port == "" {
		port = "3306"
	}
	c := mysql.NewConfig()
	c.User = u.User.Username()
	c.Passwd, _ = u.User.Password()
	c.Net, c.Addr, c.DBName = "tcp", net.JoinHostPort(u.Hostname(), port), name
	if socket != "" {
		c.Net, c.Addr = "unix", path.Clean(socket)
	}
	c.Collation = "utf8mb4_general_ci"
	c.Timeout, c.ReadTimeout, c.WriteTimeout = 5*time.Second, 15*time.Second, 15*time.Second
	switch t := u.Query().Get("tls"); t {
	case "", "false":
	case "true", "skip-verify", "preferred":
		c.TLSConfig = t
	default:
		return nil, ErrMySQL
	}
	return c, nil
}

// Open opens a B3 database: a b3.db file inside Root, or a MySQL / MariaDB address. B3 writes into it at the same
// time: with SQLite, waits up to 5 s for its locks.
func Open(src string) (*DB, error) {
	var db *sql.DB
	my := IsMySQL(src)
	if my {
		c, err := MySQLConfig(src)
		if err != nil {
			return nil, err
		}
		conn, err := mysql.NewConnector(c)
		if err != nil {
			return nil, err
		}
		db = sql.OpenDB(conn)
		db.SetMaxOpenConns(2)
		db.SetConnMaxLifetime(time.Minute)
	} else {
		if !PathOK(src) {
			return nil, ErrPath
		}
		var err error
		if db, err = sql.Open("sqlite", "file:"+src+"?_pragma=busy_timeout(5000)"); err != nil {
			return nil, err
		}
		db.SetMaxOpenConns(1)
	}
	var n int
	if err := db.QueryRow(`SELECT COUNT(*) FROM penalties`).Scan(&n); err != nil {
		db.Close()
		if my {
			return nil, errors.New("connexion MySQL de B3 impossible : " + mysqlError(err))
		}
		return nil, errors.New("pas une base B3 (table penalties introuvable) : " + err.Error())
	}
	return &DB{db, my}, nil
}

// mysqlError: the cause without the address and user of the connection.
func mysqlError(err error) string {
	var me *mysql.MySQLError
	switch {
	case errors.As(err, &me) && me.Number == 1045:
		return "identifiant ou mot de passe refusé"
	case errors.As(err, &me) && me.Number == 1049:
		return "base de données inconnue"
	case errors.As(err, &me) && me.Number == 1146:
		return "pas une base B3 (table penalties introuvable)"
	case errors.As(err, &me):
		return me.Message
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return "le serveur MySQL ne répond pas (pare-feu, ou il n'accepte que les connexions locales)"
	}
	if strings.Contains(err.Error(), "connection refused") {
		return "connexion refusée (MySQL n'écoute pas sur cette adresse ou ce port)"
	}
	return err.Error()
}

// now: the current Unix time in SQL.
func (d *DB) now() string {
	if d.my {
		return "UNIX_TIMESTAMP()"
	}
	return "CAST(strftime('%s', 'now') AS INTEGER)"
}

// Penalty types, as B3 writes them.
const (
	Ban     = "Ban"
	TempBan = "TempBan"
	Kick    = "Kick"
	Warning = "Warning"
	Notice  = "Notice"
)

type Penalty struct {
	ID       int64  `json:"id"`
	Type     string `json:"type"`
	ClientID int64  `json:"client_id"`
	GUID     string `json:"guid"`
	Name     string `json:"name"`
	Admin    string `json:"admin"`  // admin's name; "B3" for automatic penalties, "site: <account>" from the dashboard
	Origin   string `json:"origin"` // "auto" (B3's plugins: censor, spam, too many warnings), "admin" (in game) or "site"
	Duration int64  `json:"duration"`
	Inactive bool   `json:"inactive"`
	Reason   string `json:"reason"`
	Keyword  string `json:"keyword"` // B3's reason keyword (!warn spam, censor's "cuss" / "badname"…)
	TimeAdd  int64  `json:"time_add"`
	Expire   int64  `json:"time_expire"` // -1 = never
	Active   bool   `json:"active"`      // still in force (bans, temp bans, warnings)
}

// Filter of the penalty list. Empty fields = everything.
type Filter struct {
	Type   string // Ban, TempBan, Kick, Warning, Notice
	Active bool   // only the penalties still in force
	Query  string // part of a name, or a GUID
	GUID   string // one player
	Limit  int
}

const penaltyCols = `p.id, p.type, p.client_id, COALESCE(c.guid, ''), COALESCE(c.name, ''), p.admin_id, COALESCE(a.name, ''),
	p.duration, p.inactive, p.reason, p.keyword, p.data, p.time_add, p.time_expire`

func scanPenalty(sc interface{ Scan(...any) error }, now int64) (Penalty, error) {
	var p Penalty
	var adminID int64
	var admin, data string
	err := sc.Scan(&p.ID, &p.Type, &p.ClientID, &p.GUID, &p.Name, &adminID, &admin, &p.Duration, &p.Inactive, &p.Reason, &p.Keyword, &data, &p.TimeAdd, &p.Expire)
	switch {
	case data == siteTag:
		p.Admin, p.Origin = "site", "site"
	case strings.HasPrefix(data, siteTag):
		p.Admin, p.Origin = "site: "+strings.TrimPrefix(data, siteTag), "site"
	case admin != "":
		p.Admin, p.Origin = admin, "admin"
	default:
		p.Admin, p.Origin = "B3", "auto"
	}
	p.Reason = stripColors(p.Reason)
	p.Active = !p.Inactive && p.Type != Kick && p.Type != Notice && (p.Expire == -1 || p.Expire > now)
	return p, err
}

// siteTag marks, in the data column, the penalties given from the dashboard (B3 has no admin client for them).
const siteTag = "dashboard:"

// Penalties lists the penalties, most recent first.
func (d *DB) Penalties(f Filter, now time.Time) ([]Penalty, error) {
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 300
	}
	q := `SELECT ` + penaltyCols + ` FROM penalties p LEFT JOIN clients c ON c.id = p.client_id LEFT JOIN clients a ON a.id = p.admin_id
		WHERE p.type <> ''`
	var args []any
	if f.Type != "" {
		q += ` AND p.type = ?`
		args = append(args, f.Type)
	}
	if f.Active {
		q += ` AND p.inactive = 0 AND p.type NOT IN ('Kick', 'Notice') AND (p.time_expire = -1 OR p.time_expire > ?)`
		args = append(args, now.Unix())
	}
	if f.GUID != "" {
		q += ` AND c.guid = ?`
		args = append(args, f.GUID)
	}
	if s := strings.TrimSpace(f.Query); s != "" {
		q += ` AND (c.name LIKE ? OR c.guid = ? OR p.reason LIKE ?)`
		args = append(args, "%"+s+"%", s, "%"+s+"%")
	}
	q += ` ORDER BY p.time_add DESC, p.id DESC LIMIT ?`
	args = append(args, f.Limit)
	rs, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []Penalty{}
	for rs.Next() {
		p, err := scanPenalty(rs, now.Unix())
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rs.Err()
}

type Group struct {
	ID      int64  `json:"id"` // bit of clients.group_bits
	Name    string `json:"name"`
	Keyword string `json:"keyword"`
	Level   int    `json:"level"`
}

// Groups lists B3's groups, lowest level first (guest 0, user 1, reg 2, mod 20, admin 40, fulladmin 60, senioradmin 80, superadmin 100).
func (d *DB) Groups() ([]Group, error) {
	rs, err := d.Query("SELECT id, name, keyword, level FROM `groups` ORDER BY level, id") // a reserved word in MySQL 8
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Group
	for rs.Next() {
		var g Group
		if err := rs.Scan(&g.ID, &g.Name, &g.Keyword, &g.Level); err != nil {
			return nil, err
		}
		out = append(out, g)
	}
	return out, rs.Err()
}

// group of a client: the highest of the groups in its bits (guest without any).
func groupOf(bits int64, groups []Group) Group {
	best := Group{Name: "Guest", Keyword: "guest"}
	for _, g := range groups {
		if (g.ID == 0 && bits == 0) || (g.ID != 0 && bits&g.ID != 0) {
			if g.Level >= best.Level {
				best = g
			}
		}
	}
	return best
}

type Client struct {
	ID          int64  `json:"id"`
	GUID        string `json:"guid"`
	Name        string `json:"name"`
	IP          string `json:"ip,omitempty"`
	Connections int64  `json:"connections"`
	Group       Group  `json:"group"`
	Mask        *Group `json:"mask,omitempty"`     // the grade shown to the players (!mask), when it differs
	Greeting    string `json:"greeting,omitempty"` // B3's own welcome message for the player (welcome plugin)
	Banned      bool   `json:"banned"`             // has a ban or temp ban in force
	TimeAdd     int64  `json:"time_add"`
	TimeEdit    int64  `json:"time_edit"` // last connection
	Given       int64  `json:"given"`     // penalties given (admins)
}

func (d *DB) clientCols() string {
	return `id, guid, name, ip, connections, group_bits, mask_level, greeting, time_add, time_edit,
	EXISTS(SELECT 1 FROM penalties b WHERE b.client_id = clients.id AND b.type IN ('Ban', 'TempBan') AND b.inactive = 0
		AND (b.time_expire = -1 OR b.time_expire > ` + d.now() + `))`
}

func (d *DB) scanClients(rs *sql.Rows, groups []Group) ([]Client, error) {
	defer rs.Close()
	out := []Client{}
	for rs.Next() {
		var c Client
		var bits, mask int64
		if err := rs.Scan(&c.ID, &c.GUID, &c.Name, &c.IP, &c.Connections, &bits, &mask, &c.Greeting, &c.TimeAdd, &c.TimeEdit, &c.Banned); err != nil {
			return nil, err
		}
		c.Group = groupOf(bits, groups)
		if mask > 0 && int(mask) != c.Group.Level {
			for _, g := range groups {
				if g.Level == int(mask) {
					m := g
					c.Mask = &m
				}
			}
		}
		out = append(out, c)
	}
	return out, rs.Err()
}

// Client finds a player by GUID.
func (d *DB) Client(guid string) (Client, bool, error) {
	groups, err := d.Groups()
	if err != nil {
		return Client{}, false, err
	}
	rs, err := d.Query(`SELECT `+d.clientCols()+` FROM clients WHERE guid = ?`, guid)
	if err != nil {
		return Client{}, false, err
	}
	cs, err := d.scanClients(rs, groups)
	if err != nil || len(cs) == 0 {
		return Client{}, false, err
	}
	return cs[0], true, nil
}

// Admins lists the players in a group of level 20 (moderator) or more, highest first, with the penalties each gave.
func (d *DB) Admins() ([]Client, error) {
	groups, err := d.Groups()
	if err != nil {
		return nil, err
	}
	var mask int64
	for _, g := range groups {
		if g.Level >= 20 {
			mask |= g.ID
		}
	}
	rs, err := d.Query(`SELECT `+d.clientCols()+` FROM clients WHERE group_bits & ? <> 0`, mask)
	if err != nil {
		return nil, err
	}
	cs, err := d.scanClients(rs, groups)
	if err != nil {
		return nil, err
	}
	given := map[int64]int64{}
	if rs, err := d.Query(`SELECT admin_id, COUNT(*) FROM penalties WHERE admin_id > 0 GROUP BY admin_id`); err == nil {
		for rs.Next() {
			var id, n int64
			if rs.Scan(&id, &n) == nil {
				given[id] = n
			}
		}
		rs.Close()
	}
	for i := range cs {
		cs[i].Given = given[cs[i].ID]
	}
	// highest level first, then the most recently seen
	for i := 1; i < len(cs); i++ {
		for j := i; j > 0 && (cs[j].Group.Level > cs[j-1].Group.Level ||
			cs[j].Group.Level == cs[j-1].Group.Level && cs[j].TimeEdit > cs[j-1].TimeEdit); j-- {
			cs[j], cs[j-1] = cs[j-1], cs[j]
		}
	}
	return cs, nil
}

type Count struct {
	Key string `json:"key"`
	N   int64  `json:"n"`
}

type Week struct {
	Start int64            `json:"start"` // Monday 00:00 UTC
	Types map[string]int64 `json:"types"`
}

type Stats struct {
	Totals     map[string]int64 `json:"totals"`  // per type, since
	Origins    map[string]int64 `json:"origins"` // auto (B3's plugins), admin (in game), site
	ActiveBans int64            `json:"active_bans"`
	Weeks      []Week           `json:"weeks"`
	Reasons    []Count          `json:"reasons"`
	Players    []Count          `json:"players"` // most penalized
	Admins     []Count          `json:"admins"`  // most active
}

// Stats sums up the moderation since a date.
func (d *DB) Stats(since, now time.Time) (Stats, error) {
	st := Stats{Totals: map[string]int64{}, Origins: map[string]int64{}, Weeks: []Week{}, Reasons: []Count{}, Players: []Count{}, Admins: []Count{}}
	s := since.Unix()
	const week = 7 * 24 * 3600
	const monday = 4 * 24 * 3600 // 1970-01-01 is a Thursday: weeks start on Monday
	div := "/"                   // integer division of integers in SQLite
	if d.my {
		div = "DIV"
	}
	rs, err := d.Query(`SELECT (time_add - ?) `+div+` ? AS w, type, COUNT(*) FROM penalties WHERE time_add >= ? AND type <> '' GROUP BY w, type ORDER BY w`,
		monday, week, s)
	if err != nil {
		return st, err
	}
	byWeek := map[int64]*Week{}
	for rs.Next() {
		var w, n int64
		var t string
		if err := rs.Scan(&w, &t, &n); err != nil {
			rs.Close()
			return st, err
		}
		if byWeek[w] == nil {
			byWeek[w] = &Week{Start: w*week + monday, Types: map[string]int64{}}
		}
		byWeek[w].Types[t] += n
		st.Totals[t] += n
	}
	rs.Close()
	for w := (s - monday) / week; w <= (now.Unix()-monday)/week; w++ {
		if x := byWeek[w]; x != nil {
			st.Weeks = append(st.Weeks, *x)
		} else {
			st.Weeks = append(st.Weeks, Week{Start: w*week + monday, Types: map[string]int64{}})
		}
	}
	if rs, err := d.Query(`SELECT CASE WHEN data LIKE 'dashboard:%' THEN 'site' WHEN admin_id > 0 THEN 'admin' ELSE 'auto' END o, COUNT(*)
		FROM penalties WHERE time_add >= ? AND type IN ('Ban','TempBan','Kick','Warning') GROUP BY o`, s); err == nil {
		for rs.Next() {
			var o string
			var n int64
			if rs.Scan(&o, &n) == nil {
				st.Origins[o] = n
			}
		}
		rs.Close()
	}
	d.QueryRow(`SELECT COUNT(DISTINCT client_id) FROM penalties WHERE type IN ('Ban', 'TempBan') AND inactive = 0
		AND (time_expire = -1 OR time_expire > ?)`, now.Unix()).Scan(&st.ActiveBans)
	top := func(q string, dst *[]Count) error {
		rs, err := d.Query(q, s)
		if err != nil {
			return err
		}
		defer rs.Close()
		for rs.Next() {
			var c Count
			if err := rs.Scan(&c.Key, &c.N); err != nil {
				return err
			}
			*dst = append(*dst, c)
		}
		return rs.Err()
	}
	if err := top(`SELECT lower(trim(reason)) r, COUNT(*) n FROM penalties WHERE time_add >= ? AND trim(reason) <> '' AND type IN ('Ban','TempBan','Kick','Warning')
		GROUP BY r ORDER BY n DESC LIMIT 8`, &st.Reasons); err != nil {
		return st, err
	}
	if err := top(`SELECT COALESCE(c.name, '?'), COUNT(*) n FROM penalties p LEFT JOIN clients c ON c.id = p.client_id
		WHERE p.time_add >= ? AND p.type IN ('Ban','TempBan','Kick','Warning') GROUP BY p.client_id, c.name ORDER BY n DESC LIMIT 8`, &st.Players); err != nil {
		return st, err
	}
	concat := `'site: ' || substr(p.data, 11)`
	if d.my { // || is OR in MySQL
		concat = `CONCAT('site: ', SUBSTRING(p.data, 11))`
	}
	err = top(`SELECT CASE WHEN p.data = 'dashboard:' THEN 'site' WHEN p.data LIKE 'dashboard:%' THEN `+concat+`
		ELSE COALESCE(a.name, 'B3') END who, COUNT(*) n
		FROM penalties p LEFT JOIN clients a ON a.id = p.admin_id AND p.admin_id > 0
		WHERE p.time_add >= ? AND p.type IN ('Ban','TempBan','Kick','Warning') GROUP BY who ORDER BY n DESC LIMIT 8`, &st.Admins)
	return st, err
}

// ---- actions from the dashboard

// clientID finds a player, or adds them (a player banned before B3 ever saw them).
func (d *DB) clientID(guid, name, ip string, now time.Time) (int64, error) {
	var id int64
	err := d.QueryRow(`SELECT id FROM clients WHERE guid = ?`, guid).Scan(&id)
	if !errors.Is(err, sql.ErrNoRows) {
		return id, err
	}
	res, err := d.Exec(`INSERT INTO clients (guid, name, ip, time_add, time_edit) VALUES (?, ?, ?, ?, ?)`,
		guid, name, ip, now.Unix(), now.Unix())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Add writes a penalty given from the dashboard by account `by`. minutes: duration of a temp ban or a warning
// (ignored for a ban or a kick, which never expire).
func (d *DB) Add(typ, guid, name, ip string, minutes int64, reason, by string, now time.Time) (int64, error) {
	if guid == "" || guid == "0" {
		return 0, errors.New("joueur sans GUID : B3 ne peut pas le sanctionner")
	}
	expire := int64(-1)
	switch typ {
	case Ban, Kick, Notice:
		minutes = 0
	case TempBan, Warning:
		if minutes <= 0 {
			return 0, errors.New("durée manquante")
		}
		expire = now.Unix() + minutes*60
	default:
		return 0, errors.New("type de sanction inconnu")
	}
	cid, err := d.clientID(guid, name, ip, now)
	if err != nil {
		return 0, err
	}
	res, err := d.Exec(`INSERT INTO penalties (type, client_id, admin_id, duration, inactive, keyword, reason, data, time_add, time_edit, time_expire)
		VALUES (?, ?, 0, ?, 0, '', ?, ?, ?, ?, ?)`, typ, cid, minutes, reason, siteTag+by, now.Unix(), now.Unix(), expire)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Unban lifts the bans and temp bans of a player. Returns how many were lifted.
func (d *DB) Unban(guid string, now time.Time) (int64, error) {
	res, err := d.Exec(`UPDATE penalties SET inactive = 1, time_edit = ? WHERE type IN ('Ban', 'TempBan') AND inactive = 0
		AND client_id IN (SELECT id FROM clients WHERE guid = ?)`, now.Unix(), guid)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// Deactivate lifts one penalty (a warning, or a single ban).
func (d *DB) Deactivate(id int64, now time.Time) error {
	_, err := d.Exec(`UPDATE penalties SET inactive = 1, time_edit = ? WHERE id = ?`, now.Unix(), id)
	return err
}

// SetGroup puts a player in one group (B3's !putgroup). It applies at the player's next connection.
func (d *DB) SetGroup(guid string, groupID int64, now time.Time) (Group, error) {
	groups, err := d.Groups()
	if err != nil {
		return Group{}, err
	}
	var g *Group
	for i := range groups {
		if groups[i].ID == groupID {
			g = &groups[i]
		}
	}
	if g == nil {
		return Group{}, ErrNoGroup
	}
	res, err := d.Exec(`UPDATE clients SET group_bits = ? WHERE guid = ?`, groupID, guid)
	if err != nil {
		return Group{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return Group{}, ErrNoClient
	}
	return *g, nil
}

// LastID is the newest penalty: the alert watcher starts after it.
func (d *DB) LastID() (int64, error) {
	var id sql.NullInt64
	err := d.QueryRow(`SELECT MAX(id) FROM penalties`).Scan(&id)
	return id.Int64, err
}

// BansAfter lists the bans and temp bans newer than id.
func (d *DB) BansAfter(id int64, now time.Time) ([]Penalty, error) {
	rs, err := d.Query(`SELECT `+penaltyCols+` FROM penalties p LEFT JOIN clients c ON c.id = p.client_id LEFT JOIN clients a ON a.id = p.admin_id
		WHERE p.id > ? AND p.type IN ('Ban', 'TempBan') ORDER BY p.id`, id)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []Penalty
	for rs.Next() {
		p, err := scanPenalty(rs, now.Unix())
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rs.Err()
}

var colorRe = regexp.MustCompile(`\^[0-9]`)

func stripColors(s string) string { return colorRe.ReplaceAllString(s, "") }

// ---- identity: names and IPs B3 saw for a player, accounts sharing an IP, search, ban evasion by IP

type Seen struct {
	Value string `json:"value"` // the name or the IP
	Used  int64  `json:"used"`  // how many times
	First int64  `json:"first"`
	Last  int64  `json:"last"`
}

// Names lists the names a player used (B3's aliases), the current one included.
func (d *DB) Names(clientID int64) ([]Seen, error) {
	return d.seen(`SELECT alias, num_used, time_add, time_edit FROM aliases WHERE client_id = ?
		UNION ALL SELECT name, 1, time_add, time_edit FROM clients WHERE id = ? AND name NOT IN (SELECT alias FROM aliases WHERE client_id = ?)
		ORDER BY 4 DESC`, clientID, clientID, clientID)
}

// IPs lists the IPs a player connected from (B3's IP aliases), the current one included.
func (d *DB) IPs(clientID int64) ([]Seen, error) {
	return d.seen(`SELECT ip, num_used, time_add, time_edit FROM ipaliases WHERE client_id = ?
		UNION ALL SELECT ip, 1, time_add, time_edit FROM clients WHERE id = ? AND ip <> '' AND ip NOT IN (SELECT ip FROM ipaliases WHERE client_id = ?)
		ORDER BY 4 DESC`, clientID, clientID, clientID)
}

func (d *DB) seen(q string, args ...any) ([]Seen, error) {
	rs, err := d.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []Seen{}
	for rs.Next() {
		var x Seen
		if err := rs.Scan(&x.Value, &x.Used, &x.First, &x.Last); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rs.Err()
}

// all the IPs of each player: the IP aliases and the current IP (a derived table: MySQL 5.7 has no WITH)
const ips = `(SELECT client_id, ip FROM ipaliases WHERE ip <> '' UNION SELECT id, ip FROM clients WHERE ip <> '')`

type Linked struct {
	Client
	Shared []string `json:"shared"` // IPs in common
}

// Linked lists the other players who connected from one of this player's IPs.
func (d *DB) Linked(clientID int64) ([]Linked, error) {
	rs, err := d.Query(`SELECT o.client_id, group_concat(DISTINCT o.ip) FROM `+ips+` m JOIN `+ips+` o ON o.ip = m.ip AND o.client_id <> m.client_id
		WHERE m.client_id = ? GROUP BY o.client_id LIMIT 50`, clientID)
	if err != nil {
		return nil, err
	}
	shared := map[int64][]string{}
	var ids []any
	for rs.Next() {
		var id int64
		var list string
		if err := rs.Scan(&id, &list); err != nil {
			rs.Close()
			return nil, err
		}
		shared[id] = strings.Split(list, ",")
		ids = append(ids, id)
	}
	rs.Close()
	out := []Linked{}
	if len(ids) == 0 {
		return out, nil
	}
	cs, err := d.clientsWhere(`id IN (?`+strings.Repeat(",?", len(ids)-1)+`) ORDER BY time_edit DESC`, ids...)
	for _, c := range cs {
		out = append(out, Linked{Client: c, Shared: shared[c.ID]})
	}
	return out, err
}

func (d *DB) clientsWhere(where string, args ...any) ([]Client, error) {
	groups, err := d.Groups()
	if err != nil {
		return nil, err
	}
	rs, err := d.Query(`SELECT `+d.clientCols()+` FROM clients WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	return d.scanClients(rs, groups)
}

// Seen lists the players B3 saw between from and to (first visit before to, last visit after from), most recent first.
func (d *DB) Seen(from, to int64, limit int) ([]Client, error) {
	return d.clientsWhere(`guid <> '' AND time_edit >= ? AND time_add < ? ORDER BY time_edit DESC LIMIT ?`, from, to, limit)
}

// Search finds players by name (current or past), IP (or its start, "1.2.3.") or GUID, like B3's !lookup.
func (d *DB) Search(q string, limit int) ([]Client, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return []Client{}, nil
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	like := "%" + q + "%"
	return d.clientsWhere(`guid = ? OR name LIKE ? OR ip LIKE ?
		OR id IN (SELECT client_id FROM aliases WHERE alias LIKE ?) OR id IN (SELECT client_id FROM ipaliases WHERE ip LIKE ?)
		ORDER BY time_edit DESC LIMIT ?`, q, like, q+"%", like, q+"%", limit)
}

type Evasion struct {
	Banned  Client   `json:"banned"`
	BanAt   int64    `json:"ban_at"`
	BanType string   `json:"ban_type"`
	Reason  string   `json:"reason"`
	Suspect Client   `json:"suspect"`
	Shared  []string `json:"shared"`
}

// Evasions lists the players not banned who connected, after a ban in force, from an IP of the banned player:
// probably the same person with another GUID (GUID spoofer, new copy of the game).
func (d *DB) Evasions(now time.Time) ([]Evasion, error) {
	banned := `(SELECT client_id, MAX(time_add) t FROM penalties WHERE type IN ('Ban', 'TempBan') AND inactive = 0
			AND (time_expire = -1 OR time_expire > ?) GROUP BY client_id)`
	rs, err := d.Query(`SELECT b.client_id, b.t, o.client_id, group_concat(DISTINCT o.ip) FROM `+banned+` b
		JOIN `+ips+` bi ON bi.client_id = b.client_id JOIN `+ips+` o ON o.ip = bi.ip AND o.client_id <> b.client_id
		JOIN clients oc ON oc.id = o.client_id
		WHERE oc.time_edit > b.t AND o.client_id NOT IN (SELECT client_id FROM `+banned+` b2)
		GROUP BY b.client_id, b.t, o.client_id ORDER BY MAX(oc.time_edit) DESC LIMIT 100`, now.Unix(), now.Unix())
	if err != nil {
		return nil, err
	}
	type row struct {
		banned, suspect, at int64
		ips                 string
	}
	var rows []row
	for rs.Next() {
		var r row
		if err := rs.Scan(&r.banned, &r.at, &r.suspect, &r.ips); err != nil {
			rs.Close()
			return nil, err
		}
		rows = append(rows, r)
	}
	rs.Close()
	out := []Evasion{}
	for _, r := range rows {
		cs, err := d.clientsWhere(`id IN (?, ?)`, r.banned, r.suspect)
		if err != nil {
			return nil, err
		}
		e := Evasion{BanAt: r.at, Shared: strings.Split(r.ips, ",")}
		for _, c := range cs {
			if c.ID == r.banned {
				e.Banned = c
			} else {
				e.Suspect = c
			}
		}
		d.QueryRow(`SELECT type, reason FROM penalties WHERE client_id = ? AND type IN ('Ban', 'TempBan') AND time_add = ? LIMIT 1`,
			r.banned, r.at).Scan(&e.BanType, &e.Reason)
		e.Reason = stripColors(e.Reason)
		out = append(out, e)
	}
	return out, nil
}
