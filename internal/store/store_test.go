package store

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/query"
)

func TestRecordSessionsOutagesCvars(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := s.UpsertServer("x", "1.2.3.4", 1)
	pl := func(names ...string) *query.Status {
		st := &query.Status{Vars: map[string]string{"mapname": "mp_crash", "uptime": time.Now().String()}}
		for _, n := range names {
			st.Players = append(st.Players, query.Player{NameRaw: "^1" + n, NameClean: n, Score: 1})
		}
		return st
	}
	t0 := time.Unix(1000, 0)
	at := func(sec int) time.Time { return t0.Add(time.Duration(sec) * time.Second) }
	steps := []*query.Status{pl("a", "b"), pl("a"), nil, nil, pl("a")}
	for i, st := range steps {
		if err := s.Record(id, at(i*15), st); err != nil {
			t.Fatal(err)
		}
	}
	var n int
	// a: [0..15], reopened at 60; b: [0..0]
	s.DB.QueryRow(`SELECT COUNT(*) FROM player_sessions WHERE name_clean='a'`).Scan(&n)
	if n != 2 {
		t.Fatalf("want 2 sessions for a, got %d", n)
	}
	var play int
	s.DB.QueryRow(`SELECT SUM(last_seen-started_at) FROM player_sessions WHERE name_clean='a'`).Scan(&play)
	if play != 15 {
		t.Fatalf("want 15s playtime, got %d", play)
	}
	var start, end int64
	s.DB.QueryRow(`SELECT COUNT(*), MIN(started_at), MAX(ended_at) FROM outages`).Scan(&n, &start, &end)
	if n != 1 || start != at(30).Unix() || end != at(60).Unix() {
		t.Fatalf("outage: n=%d %d-%d", n, start, end)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM cvar_history`).Scan(&n)
	if n != 1 {
		t.Fatalf("cvars should be stored once when unchanged (uptime ignored), got %d", n)
	}
	if err := s.Purge(at(1000)); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow(`SELECT COUNT(*) FROM snapshots`).Scan(&n)
	if n != 0 {
		t.Fatalf("purge left %d snapshots", n)
	}
}

func TestEnterLeave(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(sec int64) time.Time { return time.Unix(sec, 0) }
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(s.Enter(1, at(100), 0, "1.1.1.1:5", "1.1.1.1", "765", "^1Bab", "Bab"))
	must(s.Leave(1, at(160), 0, "1.1.1.1:5", "231", "^1Bab", "Bab"))
	// Rejoin under another name with the same steamid: GUID is known right away.
	must(s.Enter(1, at(200), 0, "1.1.1.1:6", "1.1.1.1", "765", "Deep", "Deep"))
	var guid string
	must(s.DB.QueryRow(`SELECT guid FROM connections WHERE joined_at=200`).Scan(&guid))
	if guid != "231" {
		t.Fatalf("guid not inferred from steamid: %q", guid)
	}
	// Never left (crash): a new enter on the same slot closes it as unknown.
	must(s.Enter(1, at(300), 0, "2.2.2.2:1", "2.2.2.2", "", "Other", "Other"))
	var open, unknown int
	must(s.DB.QueryRow(`SELECT SUM(open), SUM(open=0 AND left_at IS NULL) FROM connections`).Scan(&open, &unknown))
	if open != 1 || unknown != 1 {
		t.Fatalf("open=%d unknown=%d", open, unknown)
	}
	var first, last int64
	must(s.DB.QueryRow(`SELECT first_seen, last_seen FROM player_names WHERE guid='231'`).Scan(&first, &last))
	if first != 160 || last != 160 {
		t.Fatalf("names %d-%d", first, last)
	}
}

func TestCrashClosesConnections(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(sec int64) time.Time { return time.Unix(sec, 0) }
	on := &query.Status{Vars: map[string]string{}}
	for _, err := range []error{
		s.Enter(1, at(100), 0, "a:1", "a", "", "A", "A"),
		s.Record(1, at(110), on),
		s.Record(1, at(115), on), // last answer before the crash
		s.Record(1, at(120), nil),
		s.Enter(1, at(200), 1, "b:1", "b", "", "B", "B"),
		s.Record(1, at(210), on),
		s.Record(1, at(220), nil),
		// History import case: the outage is already recorded when C's connection is read from enterleave.log,
		// C's leave line is missing and someone else takes slot 2 later: C ended at the crash.
		s.Record(1, at(240), on),
		s.Record(1, at(250), nil),
		s.Record(1, at(300), on),
		s.Enter(1, at(230), 2, "c:1", "c", "", "C", "C"),
		s.Enter(1, at(400), 2, "d:1", "d", "", "D", "D"),
		// No outage in between: end stays unknown.
		s.Enter(1, at(500), 3, "e:1", "e", "", "E", "E"),
		s.Enter(1, at(510), 3, "f:1", "f", "", "F", "F"),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]int64{}
	rows, _ := s.DB.Query(`SELECT name_clean, COALESCE(left_at, -1) FROM connections WHERE open=0`)
	for rows.Next() {
		var n string
		var l int64
		rows.Scan(&n, &l)
		got[n] = l
	}
	if got["A"] != 115 || got["B"] != 210 || got["C"] != 240 || got["E"] != -1 {
		t.Fatalf("crash times: %v", got)
	}
}

func TestKillsAndRounds(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	at := func(sec int64) time.Time { return time.Unix(sec, 0) }
	for _, err := range []error{
		s.StartRound(1, at(0), "mp_crash", "sd"),
		s.AddKill(1, at(10), "111", "^1A", "A", "axis", "222", "B", "B", "allies", "m40a3_mp", "MOD_HEAD_SHOT", "head", 100),
		s.AddKill(1, at(20), "", "", "", "", "222", "B", "B", "allies", "none", "MOD_SUICIDE", "none", 100000), // suicide
		s.AddKill(1, at(30), "111", "^1A", "A", "axis", "0", "Noob", "Noob", "axis", "m40a3_mp", "MOD_RIFLE_BULLET", "torso_upper", 90),
		s.StartRound(1, at(60), "mp_crash", "sd"),
		s.EndRound(1, at(90)),
	} {
		if err != nil {
			t.Fatal(err)
		}
	}
	var rounds, open int
	s.DB.QueryRow(`SELECT COUNT(*), SUM(ended_at IS NULL) FROM rounds`).Scan(&rounds, &open)
	if rounds != 1 || open != 0 { // the second round had nothing in it: dropped at the end of the map

		t.Fatalf("rounds=%d open=%d", rounds, open)
	}
	var inFirst, suicides, tk int
	var noGUIDKey string
	s.DB.QueryRow(`SELECT COUNT(*) FROM kills WHERE round_id=(SELECT MIN(id) FROM rounds)`).Scan(&inFirst)
	s.DB.QueryRow(`SELECT COUNT(*) FROM kills WHERE killer_key IS NULL`).Scan(&suicides)
	s.DB.QueryRow(`SELECT SUM(teamkill), MAX(IIF(victim_guid IS NULL, victim_key, NULL)) FROM kills`).Scan(&tk, &noGUIDKey)
	if inFirst != 3 || suicides != 1 || tk != 1 || noGUIDKey != "n:Noob" {
		t.Fatalf("inFirst=%d suicides=%d tk=%d key=%q", inFirst, suicides, tk, noGUIDKey)
	}
}

// CoD4X logs InitGame when the map loads, not for every S&D round: the mod's RE line splits the rounds.
func TestRoundsFromRoundEnd(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	base := time.Unix(1_000_000, 0)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }
	kill := func(s int) {
		if err := st.AddKill(1, at(s), "a", "a", "a", "axis", "b", "b", "b", "allies", "m40a3_mp", "MOD_RIFLE_BULLET", "torso_upper", 100); err != nil {
			t.Fatal(err)
		}
	}
	st.StartRound(1, at(0), "mp_crash", "sd")
	kill(10)
	kill(20)
	st.FinishRound(1, at(100), "axis", 0, 1, "axis") // round 1: 2 kills
	st.StartRound(1, at(105), "mp_crash", "sd")      // a server that logs InitGame every round: same round 2
	kill(130)
	st.FinishRound(1, at(200), "allies", 1, 1, "axis") // round 2: 1 kill
	st.FinishRound(1, at(300), "allies", 2, 1, "axis") // round 3: no kill (bomb), still a round
	st.EndRound(1, at(320))                            // end of the map: the round opened after the last RE is empty, dropped
	st.StartRound(1, at(330), "mp_backlot", "sd")

	rs, err := st.DB.Query(`SELECT r.map, COALESCE(r.winner, ''), r.ended_at IS NULL, (SELECT COUNT(*) FROM kills k WHERE k.round_id=r.id) FROM rounds r ORDER BY r.id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rs.Close()
	var got []string
	for rs.Next() {
		var m, w string
		var open bool
		var n int
		rs.Scan(&m, &w, &open, &n)
		got = append(got, fmt.Sprintf("%s/%s/%v/%d", m, w, open, n))
	}
	want := "mp_crash/axis/false/2 mp_crash/allies/false/1 mp_crash/allies/false/0 mp_backlot//true/0"
	if strings.Join(got, " ") != want {
		t.Fatalf("rounds:\n got %s\nwant %s", strings.Join(got, " "), want)
	}
}

func TestMeCode(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1_000_000, 0)
	a, err := s.MeCode(1, "111", "^1Babou", now)
	if err != nil || len(a) != 5 {
		t.Fatalf("code %q %v", a, err)
	}
	if again, _ := s.MeCode(1, "111", "^2Babou", now.Add(time.Hour)); again != a {
		t.Fatalf("new code while the first is valid: %q %q", a, again)
	}
	if b, _ := s.MeCode(2, "111", "Babou", now); b == a {
		t.Fatal("same code on another server")
	}
	id, key, name, ok := s.ClaimMe(" "+strings.ToLower(a[:2])+" "+a[2:], now.Add(2*time.Hour))
	if !ok || id != 1 || key != "111" || name != "^2Babou" {
		t.Fatalf("claim: %v %q %q %v", id, key, name, ok)
	}
	if _, _, _, ok := s.ClaimMe(a, now.Add(MeCodeTTL+time.Minute)); ok {
		t.Fatal("expired code accepted")
	}
	if c, _ := s.MeCode(1, "111", "Babou", now.Add(MeCodeTTL+time.Minute)); c == a {
		t.Fatal("expired code given again")
	}
}
