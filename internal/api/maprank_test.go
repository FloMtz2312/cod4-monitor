package api

import (
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestRankMaps(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	snap := func(ts int64, online int, mp string, players int) {
		if _, err := st.DB.Exec(`INSERT INTO snapshots(server_id, ts, online, map, players) VALUES(1,?,?,?,?)`, ts, online, mp, players); err != nil {
			t.Fatal(err)
		}
	}
	// backlot 5 min, 4 then 6 players; crash 5 min, 2 players leaving to 0; backlot again after the monitor was stopped 20 min
	for i := int64(0); i <= 300; i += 5 {
		snap(1000+i, 1, "mp_backlot", 4+2*int(i/300))
	}
	for i := int64(5); i <= 300; i += 5 {
		snap(1300+i, 1, "mp_crash", 2-2*int(i/300))
	}
	snap(1605, 0, "", 0) // offline
	snap(2800, 1, "mp_backlot", 3)
	snap(2805, 1, "mp_backlot", 3)
	st.AddMapVote(1, time.Unix(2810, 0), "mp_shipment", "sd", 0, false)

	ranks, err := rankMaps(st.DB, "1", 0, 5000)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]*mapRank{}
	for _, r := range ranks {
		by[r.Map] = r
	}
	b, c, s := by["mp_backlot"], by["mp_crash"], by["mp_shipment"]
	if b == nil || c == nil || s == nil || len(ranks) != 3 {
		t.Fatalf("ranks: %+v", ranks)
	}
	if b.Plays != 2 || b.Secs != 305 || b.MaxPlayers != 6 || b.AvgChange != 2.0 {
		t.Fatalf("backlot: %+v", b)
	}
	if c.Plays != 1 || c.Secs != 295 || c.AvgPlayers != 2 || c.AvgChange != -2.0 || c.EmptySecs != 0 {
		t.Fatalf("crash: %+v", c)
	}
	if s.Plays != 0 || s.Offered != 1 {
		t.Fatalf("shipment: %+v", s)
	}
	if ranks[0].Map != "mp_backlot" {
		t.Fatalf("order: %s first, want backlot (%s)", ranks[0].Map, strconv.FormatInt(ranks[0].PlayerSecs, 10))
	}
}
