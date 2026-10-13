package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSecretsMigrationAndVerifiedRestore(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "test.db")
	s, e := Open(p)
	if e != nil {
		t.Fatal(e)
	}
	id, e := s.SaveServerConf(ServerConf{Name: "A", Host: "127.0.0.1", Port: 28960, LogURL: "ftp://user:ftpsecret@host/mods/m/", RconPass: "rconsecret", Enabled: true})
	if e != nil {
		t.Fatal(e)
	}
	var raw string
	s.DB.QueryRow(`SELECT log_url||rcon_pass FROM servers WHERE id=?`, id).Scan(&raw)
	if strings.Contains(raw, "ftpsecret") || strings.Contains(raw, "rconsecret") || !strings.Contains(raw, sealedPrefix) {
		t.Fatal("plaintext secret persisted")
	}
	c, ok, e := s.ServerConf(id)
	if e != nil || !ok || c.RconPass != "rconsecret" || !strings.Contains(c.LogURL, "ftpsecret") {
		t.Fatalf("roundtrip: %+v %v", c, e)
	}
	// Simulate legacy plaintext values before reopening.
	s.DB.Exec(`UPDATE servers SET rcon_pass='legacy-secret' WHERE id=?`, id)
	s.DB.Close()
	s, e = Open(p)
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	s.DB.QueryRow(`SELECT rcon_pass FROM servers WHERE id=?`, id).Scan(&raw)
	if !strings.HasPrefix(raw, sealedPrefix) {
		t.Fatal("migration missing")
	}
	backup := filepath.Join(dir, "backup.db")
	if e = s.Backup(backup); e != nil {
		t.Fatal(e)
	}
	if e = s.VerifyBackup(backup); e != nil {
		t.Fatal(e)
	}
	if info, e := os.Stat(backup); e != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("backup permissions: %v %v", info, e)
	}
	wrong := &Store{DB: s.DB, key: make([]byte, 32)}
	if e = wrong.VerifyBackup(backup); e == nil {
		t.Fatal("wrong key accepted")
	}
	os.WriteFile(backup, []byte("corrupt"), 0600)
	if e = s.VerifyBackup(backup); e == nil {
		t.Fatal("corrupt restore accepted")
	}
}
func TestAggregatesAndRoundIndex(t *testing.T) {
	s, e := Open(filepath.Join(t.TempDir(), "t.db"))
	if e != nil {
		t.Fatal(e)
	}
	defer s.DB.Close()
	now := time.Now()
	if e = s.AddKill(1, now, "a", "A", "A", "axis", "b", "B", "B", "allies", "ak47_mp", "MOD_HEAD_SHOT", "head", 100); e != nil {
		t.Fatal(e)
	}
	var kills, hs, deaths int
	s.DB.QueryRow(`SELECT kills,headshots FROM daily_combat WHERE key='a'`).Scan(&kills, &hs)
	s.DB.QueryRow(`SELECT deaths FROM daily_combat WHERE key='b'`).Scan(&deaths)
	if kills != 1 || hs != 1 || deaths != 1 {
		t.Fatalf("%d %d %d", kills, hs, deaths)
	}
	var a, b, c int
	var plan string
	if e = s.DB.QueryRow(`EXPLAIN QUERY PLAN SELECT COUNT(*) FROM kills WHERE round_id=123`).Scan(&a, &b, &c, &plan); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(plan, "kills_round") {
		t.Fatalf("round query scans all kills: %s", plan)
	}
}
