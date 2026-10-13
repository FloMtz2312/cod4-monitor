package store

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestServerConf(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if has, _ := s.HasManagedServers(); has {
		t.Fatal("no server yet")
	}
	old, _ := s.UpsertServer("Ancien", "1.1.1.1", 28960) // seen before, history only
	id, err := s.SaveServerConf(ServerConf{Name: "A", Host: "1.1.1.1", Port: 28960, RconPass: "x", Enabled: true})
	if err != nil || id != old {
		t.Fatalf("take over the old row: id=%d old=%d err=%v", id, old, err)
	}
	if _, err := s.SaveServerConf(ServerConf{Name: "B", Host: "1.1.1.1", Port: 28960, Enabled: true}); !errors.Is(err, ErrServerExists) {
		t.Fatalf("duplicate address: %v", err)
	}
	b, err := s.SaveServerConf(ServerConf{Name: "B", Host: "2.2.2.2", Port: 28960, LogURL: "ftp://u:p@h/mods/m/", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetServerEnabled(b, false); err != nil {
		t.Fatal(err)
	}
	c, ok, err := s.ServerConf(b)
	if err != nil || !ok || c.Enabled || !c.Managed || c.LogURL != "ftp://u:p@h/mods/m/" {
		t.Fatalf("conf: %+v %v %v", c, ok, err)
	}
	if has, _ := s.HasManagedServers(); !has {
		t.Fatal("managed servers expected")
	}
	if _, ok, _ := s.ServerConf(999); ok {
		t.Fatal("unknown server found")
	}
}
