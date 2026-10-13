package api

import (
	"path/filepath"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestWeekSummary(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	base := time.Unix(1_000_000, 0)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }
	kill := func(s int, killer, victim, cause string) {
		if err := st.AddKill(1, at(s), killer, "^1"+killer, killer, "axis", victim, victim, victim, "allies", "m40a3_mp", cause, "head", 100); err != nil {
			t.Fatal(err)
		}
	}
	st.StartRound(1, at(0), "mp_crash", "sd")
	kill(10, "alice", "bob", "MOD_HEAD_SHOT")
	kill(20, "alice", "carl", "MOD_RIFLE_BULLET")
	kill(30, "bob", "alice", "MOD_HEAD_SHOT")
	st.SetLastKillExtra(1, store.KillExtra{Distance: 87.5, Tags: "ns,hs"})
	for _, p := range []struct {
		name string
		a, b int
	}{{"alice", 0, 600}, {"bob", 100, 400}, {"alice", 700, 800}} {
		st.DB.Exec(`INSERT INTO player_sessions(server_id, name_raw, name_clean, started_at, last_seen, open) VALUES(1,?,?,?,?,0)`, p.name, p.name, at(p.a).Unix(), at(p.b).Unix())
	}
	st.DB.Exec(`INSERT INTO snapshots(server_id, ts, online, map, players) VALUES(1,?,1,'mp_crash',3),(1,?,1,'mp_crash',5),(1,?,1,'mp_crash',2)`, at(0).Unix(), at(5).Unix(), at(10).Unix())

	w, err := WeekSummary(st.DB, "1", at(0).Unix(), at(1000).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if w.Players != 2 || w.PlaySecs != 1000 || w.Peak != 5 || w.Kills != 3 || w.Matches != 1 || w.Map != "mp_crash" {
		t.Fatalf("totals: %+v", w)
	}
	if len(w.Top) != 2 || w.Top[0].Key != "alice" || w.Top[0].Kills != 2 || w.Top[0].Deaths != 1 || w.Top[0].Headshots != 1 || w.Top[0].Name != "^1alice" {
		t.Fatalf("top: %+v", w.Top)
	}
	if w.Longest == nil || w.Longest.Key != "bob" || w.Longest.Value != 87.5 || w.Noscopes == nil || w.Noscopes.Value != 1 || w.Headshots == nil {
		t.Fatalf("records: %+v %+v %+v", w.Longest, w.Noscopes, w.Headshots)
	}
	from, to := LastWeek(time.Date(2026, 10, 7, 16, 0, 0, 0, time.UTC), time.UTC) // a Wednesday
	if from.Format("2006-01-02 Mon") != "2026-09-28 Mon" || to.Format("2006-01-02") != "2026-10-05" {
		t.Fatalf("last week: %v %v", from, to)
	}
}
