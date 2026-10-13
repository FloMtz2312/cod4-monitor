package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cod4monitor/internal/api"
	"cod4monitor/internal/store"
)

func TestFleetStartStop(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := importServers(st, "A|127.0.0.1:1|ftp://u:p@127.0.0.1:1/mods/m/|pw;B|127.0.0.1:2"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &fleet{ctx: ctx, st: st, hub: api.NewHub(), interval: time.Hour, timeout: 50 * time.Millisecond, logEvery: time.Hour,
		offlineAfter: 3, loc: time.UTC, run: map[int64]*running{}}
	for _, id := range []int64{1, 2} {
		if err := f.Apply(id); err != nil {
			t.Fatal(err)
		}
	}
	if f.Count() != 2 || f.Rcon(1) == nil || f.Rcon(2) != nil || f.Tail(1) == nil || f.Tail(2) != nil {
		t.Fatalf("running: %d rcon1=%v tail1=%v", f.Count(), f.Rcon(1) != nil, f.Tail(1) != nil)
	}
	st.SetServerEnabled(1, false)
	if err := f.Apply(1); err != nil || f.Count() != 1 || f.Rcon(1) != nil {
		t.Fatalf("after disabling: count=%d err=%v", f.Count(), err)
	}
	cancel()
	done := make(chan struct{})
	go func() { f.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("fleet did not stop")
	}
}
