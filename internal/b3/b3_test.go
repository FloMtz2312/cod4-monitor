package b3

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

// newDB creates a B3 database with B3's own schema (testdata/b3.sql, from B3 1.10.10), in SQLite.
func newDB(t *testing.T) (*DB, string) {
	t.Helper()
	dir := t.TempDir()
	old := Root
	t.Cleanup(func() { Root = old })
	SetRoot(dir)
	p := filepath.Join(dir, "b3.db")
	schema, err := os.ReadFile("testdata/b3.sql")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	raw.Close()
	d, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d, p
}

// newMySQL empties the MySQL / MariaDB database of B3_TEST_MYSQL (mysql://user:pass@host:port/db) and creates B3's
// MySQL schema there (testdata/b3-mysql.sql). Without B3_TEST_MYSQL, the MySQL variant of the tests is skipped.
func newMySQL(t *testing.T) *DB {
	t.Helper()
	src := os.Getenv("B3_TEST_MYSQL")
	if src == "" {
		t.Skip("B3_TEST_MYSQL not set: no MySQL server to test with")
	}
	c, err := MySQLConfig(src)
	if err != nil {
		t.Fatal(err)
	}
	c.MultiStatements = true
	conn, err := mysql.NewConnector(c)
	if err != nil {
		t.Fatal(err)
	}
	raw := sql.OpenDB(conn)
	defer raw.Close()
	schema, err := os.ReadFile("testdata/b3-mysql.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec("DROP TABLE IF EXISTS aliases, ipaliases, clients, `groups`, penalties, data; " + string(schema)); err != nil {
		t.Fatal(err)
	}
	d, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// both runs a test on SQLite, then on MySQL when B3_TEST_MYSQL is set.
func both(t *testing.T, test func(t *testing.T, d *DB)) {
	t.Run("sqlite", func(t *testing.T) { d, _ := newDB(t); test(t, d) })
	t.Run("mysql", func(t *testing.T) { test(t, newMySQL(t)) })
}

func TestPaths(t *testing.T) {
	_, p := newDB(t)
	if PathOK("/etc/passwd") || PathOK(filepath.Join(filepath.Dir(p), "../x.db")) || !PathOK(p) {
		t.Fatal("PathOK")
	}
	for src, ok := range map[string]bool{
		"mysql://b3:pw@db.example:3306/b3":                                 true,
		"mysql://b3@db.example/b3?tls=true":                                true,
		"mysql://b3:pw@db.example/":                                        false,
		"mysql://:pw@db.example/b3":                                        false,
		"mysql://b3:pw@db.example/b3?tls=maybe":                            false,
		"mysql://b3:pw@db.example/b3?allowAllFiles=true":                   true, // unknown parameters are ignored, never passed on
		"mysql://b3:p%40ss@[2001:db8::1]:3307/b3_database":                 true,
		"mysql://b3:pw@localhost/b3?socket=" + Root + "mysqld/mysqld.sock": true,
		"mysql://b3:pw@localhost/b3?socket=/run/mysqld/mysqld.sock":        false, // outside B3_ROOT
	} {
		c, err := MySQLConfig(src)
		if (err == nil) != ok {
			t.Errorf("%s: %v", src, err)
		}
		if err == nil && c.AllowAllFiles {
			t.Errorf("%s: LOAD DATA LOCAL allowed", src)
		}
	}
	if c, _ := MySQLConfig("mysql://b3:p%40ss@[2001:db8::1]:3307/b3_database"); c.Passwd != "p@ss" || c.Addr != "[2001:db8::1]:3307" || c.DBName != "b3_database" {
		t.Fatalf("parsed %+v", c)
	}
	if c, _ := MySQLConfig("mysql://b3:pw@localhost/b3?socket=" + Root + "mysqld/mysqld.sock"); c.Net != "unix" || c.Addr != Root+"mysqld/mysqld.sock" {
		t.Fatalf("socket %+v", c)
	}
}

func TestB3(t *testing.T) { both(t, testB3) }

func testB3(t *testing.T, d *DB) {
	now := time.Unix(1_800_000_000, 0)
	// players seen by B3: an admin (group 16 = Admin, level 40) and a player
	if _, err := d.Exec(`INSERT INTO clients (guid, name, ip, connections, group_bits, time_add, time_edit) VALUES
		('111', 'Boss', '1.1.1.1', 50, 16, 1, 2), ('222', 'Cheater', '2.2.2.2', 3, 1, 1, 3)`); err != nil {
		t.Fatal(err)
	}
	// the players seen between two dates: last visit after from, first visit before to
	if cs, err := d.Seen(3, 10, 10); err != nil || len(cs) != 1 || cs[0].GUID != "222" || cs[0].Connections != 3 {
		t.Fatalf("seen %+v %v", cs, err)
	}
	// a warning given by the admin in game, a ban from the dashboard for an unknown player
	if _, err := d.Exec(`INSERT INTO penalties (type, client_id, admin_id, duration, reason, time_add, time_expire) VALUES
		('Warning', 2, 1, 60, 'spam', ?, ?)`, now.Unix()-100, now.Unix()+3500); err != nil {
		t.Fatal(err)
	}
	last, _ := d.LastID()
	if _, err := d.Add(TempBan, "333", "New", "3.3.3.3", 0, "x", "admin", now); err == nil {
		t.Fatal("temp ban without a duration accepted")
	}
	if _, err := d.Add(Ban, "0", "NoID", "", 0, "x", "admin", now); err == nil {
		t.Fatal("ban of a player without id accepted")
	}
	if _, err := d.Add(TempBan, "333", "New", "3.3.3.3", 120, "aimbot", "florian", now); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Add(Kick, "222", "Cheater", "", 0, "afk", "florian", now); err != nil {
		t.Fatal(err)
	}

	all, err := d.Penalties(Filter{}, now)
	if err != nil || len(all) != 3 {
		t.Fatalf("penalties %+v %v", all, err)
	}
	if all[0].Type != Kick || all[0].Active || all[0].Admin != "site: florian" {
		t.Fatalf("kick %+v", all[0])
	}
	if w := all[2]; w.Type != Warning || !w.Active || w.Admin != "Boss" || w.Name != "Cheater" {
		t.Fatalf("warning %+v", w)
	}
	active, _ := d.Penalties(Filter{Active: true}, now)
	if len(active) != 2 { // the temp ban and the warning
		t.Fatalf("active %+v", active)
	}
	if later, _ := d.Penalties(Filter{Active: true}, now.Add(3*time.Hour)); len(later) != 0 {
		t.Fatalf("expired still active %+v", later)
	}
	if mine, _ := d.Penalties(Filter{GUID: "333"}, now); len(mine) != 1 || mine[0].Name != "New" || mine[0].Expire != now.Unix()+7200 {
		t.Fatalf("player %+v", mine)
	}
	// alert watcher: the new temp ban only
	bans, err := d.BansAfter(last, now)
	if err != nil || len(bans) != 1 || bans[0].GUID != "333" {
		t.Fatalf("bans after %+v %v", bans, err)
	}

	if n, err := d.Unban("333", now); err != nil || n != 1 {
		t.Fatalf("unban %d %v", n, err)
	}
	if active, _ := d.Penalties(Filter{Active: true, Type: TempBan}, now); len(active) != 0 {
		t.Fatal("still banned")
	}

	// groups and admins
	c, ok, err := d.Client("111")
	if err != nil || !ok || c.Group.Level != 40 || c.Group.Keyword != "admin" {
		t.Fatalf("client %+v %v", c, err)
	}
	if _, err := d.SetGroup("222", 999, now); err != ErrNoGroup {
		t.Fatalf("unknown group: %v", err)
	}
	if _, err := d.SetGroup("nobody", 8, now); err != ErrNoClient {
		t.Fatalf("unknown player: %v", err)
	}
	if g, err := d.SetGroup("222", 8, now); err != nil || g.Level != 20 {
		t.Fatalf("set group %+v %v", g, err)
	}
	admins, err := d.Admins()
	if err != nil || len(admins) != 2 || admins[0].GUID != "111" || admins[0].Given != 1 || admins[1].Group.Keyword != "mod" {
		t.Fatalf("admins %+v %v", admins, err)
	}

	st, err := d.Stats(now.AddDate(0, 0, -30), now)
	if err != nil || st.Totals[Warning] != 1 || st.Totals[TempBan] != 1 || st.Totals[Kick] != 1 || len(st.Weeks) < 4 {
		t.Fatalf("stats %+v %v", st, err)
	}
	if len(st.Players) == 0 || st.Players[0].Key != "Cheater" || st.Players[0].N != 2 {
		t.Fatalf("top players %+v", st.Players)
	}
}

func TestIdentity(t *testing.T) { both(t, testIdentity) }

func testIdentity(t *testing.T, d *DB) {
	now := time.Now()
	n := now.Unix()
	// 1 banned yesterday from 9.9.9.9; 2 came back today from the same IP with another GUID; 3 unrelated
	if _, err := d.Exec(`INSERT INTO clients (guid, name, ip, connections, group_bits, mask_level, greeting, time_add, time_edit) VALUES
		('100', 'Banned', '9.9.9.9', 5, 1, 0, '', ?, ?), ('200', 'NewGuy', '9.9.9.9', 1, 1, 0, '', ?, ?), ('300', 'Admin', '7.7.7.7', 9, 16, 1, 'Salut %name%', ?, ?)`,
		n-90000, n-86400, n-3600, n-60, n-90000, n); err != nil {
		t.Fatal(err)
	}
	d.Exec(`INSERT INTO aliases (alias, num_used, client_id, time_add, time_edit) VALUES ('OldName', 3, 1, ?, ?)`, n-90000, n-89000)
	d.Exec(`INSERT INTO ipaliases (ip, num_used, client_id, time_add, time_edit) VALUES ('8.8.8.8', 2, 1, ?, ?)`, n-90000, n-89000)
	d.Exec(`INSERT INTO penalties (type, client_id, admin_id, reason, keyword, time_add, time_expire) VALUES ('Ban', 1, 3, '^1cheat', 'cheat', ?, -1),
		('Warning', 2, 0, 'langage', 'cuss', ?, ?)`, n-86000, n-100, n+3600)

	names, _ := d.Names(1)
	if len(names) != 2 {
		t.Fatalf("names %+v", names)
	}
	ips, _ := d.IPs(1)
	if len(ips) != 2 {
		t.Fatalf("ips %+v", ips)
	}
	linked, err := d.Linked(1)
	if err != nil || len(linked) != 1 || linked[0].GUID != "200" || linked[0].Shared[0] != "9.9.9.9" {
		t.Fatalf("linked %+v %v", linked, err)
	}
	if r, _ := d.Search("OldName", 0); len(r) != 1 || r[0].GUID != "100" || !r[0].Banned {
		t.Fatalf("search by old name %+v", r)
	}
	if r, _ := d.Search("9.9.9.", 0); len(r) != 2 {
		t.Fatalf("search by IP %+v", r)
	}
	ev, err := d.Evasions(now)
	if err != nil || len(ev) != 1 || ev[0].Banned.GUID != "100" || ev[0].Suspect.GUID != "200" || ev[0].Reason != "cheat" {
		t.Fatalf("evasions %+v %v", ev, err)
	}
	ps, _ := d.Penalties(Filter{}, now)
	for _, p := range ps {
		if (p.Type == Warning && (p.Origin != "auto" || p.Keyword != "cuss")) || (p.Type == Ban && p.Origin != "admin") {
			t.Fatalf("origin %+v", p)
		}
	}
	a, _, _ := d.Client("300")
	if a.Mask == nil || a.Mask.Level != 1 || a.Greeting != "Salut %name%" {
		t.Fatalf("mask / greeting %+v", a)
	}
	st, _ := d.Stats(now.AddDate(0, 0, -7), now)
	if st.Origins["auto"] != 1 || st.Origins["admin"] != 1 {
		t.Fatalf("origins %+v", st.Origins)
	}
}
