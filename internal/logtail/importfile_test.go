package logtail

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func TestImportFiles(t *testing.T) {
	_, st, _ := recoveryFixture(t)
	ctx := context.Background()
	el := func(ts, name, verb string, slot, id string) string {
		return ts + ": ^7Client " + name + " 1.2.3.4:28960 ^7" + verb + " slot " + slot + " with playerid " + id
	}
	enterleave := strings.Join([]string{
		el("Tue Sep  1 20:00:30 2026", "Alpha", "entered this server in", "0", "0"),
		el("Tue Sep  1 20:01:00 2026", "Bravo", "entered this server in", "1", "0"),
		strings.Repeat("\x00", 100000), // junk left by a crash
		el("Tue Sep  1 20:03:00 2026", "Bravo", "left this server from", "1", "222"),
		el("Tue Sep  1 20:10:00 2026", "Alpha", "left this server from", "0", "111"),
	}, "\n") + "\n"
	if k := DetectKind(strings.NewReader(enterleave), time.UTC); k != "enterleave" {
		t.Fatalf("kind %q", k)
	}
	var p ImportProgress
	if err := ImportEnterLeave(ctx, st, 1, ImportKey(1), time.UTC, strings.NewReader(enterleave), &p, func() {}); err != nil {
		t.Fatal(err)
	}
	if p.Imported != 2 || p.Skipped != 0 {
		t.Fatalf("enterleave %+v", p)
	}
	var left int64
	var guid string
	st.DB.QueryRow(`SELECT left_at, guid FROM connections WHERE name_clean='Bravo'`).Scan(&left, &guid)
	if left != time.Date(2026, 9, 1, 20, 3, 0, 0, time.UTC).Unix() || guid != "222" {
		t.Fatalf("Bravo left %d guid %q", left, guid)
	}
	// the same file again: nothing twice
	p = ImportProgress{}
	ImportEnterLeave(ctx, st, 1, ImportKey(2), time.UTC, strings.NewReader(enterleave), &p, func() {})
	if p.Imported != 0 || p.Skipped != 2 {
		t.Fatalf("enterleave again %+v", p)
	}

	games := strings.Join([]string{
		"  0:00 InitGame: \\mapname\\mp_crash\\g_gametype\\sd",
		"  0:30 J;111;0;Alpha",
		"  1:00 J;222;1;Bravo",
		"  2:00 K;222;1;axis;Bravo;111;0;allies;Alpha;m40a3_mp;100;MOD_HEAD_SHOT;head",
		"  2:30 say;111;0;Alpha;gg",
		"  3:00 Q;222;1;Bravo",
	}, "\n") + "\n"
	if k := DetectKind(strings.NewReader(games), time.UTC); k != "games" {
		t.Fatalf("kind %q", k)
	}
	open := func() (io.ReadCloser, error) { return io.NopCloser(strings.NewReader(games)), nil }
	p = ImportProgress{}
	if err := ImportGames(ctx, st, 1, ImportKey(3), open, &p, func() {}); err != nil {
		t.Fatal(err)
	}
	var ts int64
	if err := st.DB.QueryRow(`SELECT ts FROM kills WHERE killer_key='111'`).Scan(&ts); err != nil || ts != time.Date(2026, 9, 1, 20, 2, 0, 0, time.UTC).Unix() {
		t.Fatalf("kill at %v: %v (%+v)", time.Unix(ts, 0).UTC(), err, p)
	}
	p = ImportProgress{}
	ImportGames(ctx, st, 1, ImportKey(4), open, &p, func() {})
	var n int
	st.DB.QueryRow(`SELECT COUNT(*) FROM kills`).Scan(&n)
	if n != 1 || p.Skipped != 2 {
		t.Fatalf("games again: %d kills, %+v", n, p)
	}
}
