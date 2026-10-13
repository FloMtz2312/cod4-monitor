package logtail

import (
	"cod4monitor/internal/store"
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func recoveryFixture(t *testing.T) (*Tail, *store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	old := LocalRoot
	SetLocalRoot(dir)
	t.Cleanup(func() { LocalRoot = old })
	st, e := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { st.DB.Close() })
	return &Tail{ServerID: 1, Dir: &url.URL{Scheme: "file", Path: dir + "/"}}, st, filepath.Join(dir, "events.log")
}
func chatFile(t *testing.T) *file {
	return &file{name: "events.log", persist: true, fromStart: true, handle: func(st *store.Store, lines []string, _ bool) {
		for _, line := range lines {
			st.AddChat(1, time.Now(), "1", "Player", "Player", line, false)
		}
	}}
}
func countChats(t *testing.T, st *store.Store) int {
	t.Helper()
	var n int
	if e := st.DB.QueryRow(`SELECT COUNT(*) FROM chat_messages`).Scan(&n); e != nil {
		t.Fatal(e)
	}
	return n
}
func TestResumePartialRestartAndRotation(t *testing.T) {
	tl, st, path := recoveryFixture(t)
	os.WriteFile(path, []byte("one\npar"), 0600)
	f := chatFile(t)
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if n := countChats(t, st); n != 1 {
		t.Fatal(n)
	}
	off, _ := st.LogOffset(1, "events.log")
	if off != 4 {
		t.Fatalf("partial checkpoint=%d", off)
	}
	// Restart with the incomplete line still on disk, then complete it.
	f = chatFile(t)
	os.WriteFile(path, []byte("one\npartial\ntwo\n"), 0600)
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if n := countChats(t, st); n != 3 {
		t.Fatalf("duplicate or lost lines: %d", n)
	}
	// Replacement longer than the previous file must not be mistaken for append.
	os.WriteFile(path, []byte("new first line longer than the previous file\nnew second\n"), 0600)
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if n := countChats(t, st); n != 5 {
		t.Fatalf("rotation lost lines: %d", n)
	}
	// Truncate to a short file and restart again.
	os.WriteFile(path, []byte("x\n"), 0600)
	f = chatFile(t)
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if n := countChats(t, st); n != 6 {
		t.Fatal(n)
	}
}
func TestFailedBatchDoesNotAdvanceOrEmit(t *testing.T) {
	tl, st, path := recoveryFixture(t)
	os.WriteFile(path, []byte("one\ntwo\n"), 0600)
	st.DB.Exec(`CREATE TRIGGER reject_second BEFORE INSERT ON chat_messages WHEN NEW.message='two' BEGIN SELECT RAISE(ABORT,'test failure'); END`)
	emitted := 0
	f := chatFile(t)
	orig := f.handle
	f.handle = func(s *store.Store, l []string, b bool) {
		orig(s, l, b)
		s.AfterCommit(func(*store.Store) { emitted++ })
	}
	if e := tl.poll(localSource{}, st, f); e == nil {
		t.Fatal("expected write failure")
	}
	if n := countChats(t, st); n != 0 {
		t.Fatalf("partial transaction: %d", n)
	}
	off, _ := st.LogOffset(1, "events.log")
	if off != -1 || emitted != 0 {
		t.Fatalf("checkpoint=%d emitted=%d", off, emitted)
	}
	st.DB.Exec(`DROP TRIGGER reject_second`)
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if n := countChats(t, st); n != 2 || emitted != 1 {
		t.Fatalf("retry count=%d emitted=%d", n, emitted)
	}
}
func TestFirstRunPersistsEOF(t *testing.T) {
	tl, st, path := recoveryFixture(t)
	os.WriteFile(path, []byte("old\n"), 0600)
	f := chatFile(t)
	f.fromStart = false
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	off, _ := st.LogOffset(1, "events.log")
	if off != 4 {
		t.Fatal(off)
	}
	os.WriteFile(path, []byte("old\nnew\n"), 0600)
	f = chatFile(t)
	f.fromStart = false
	if e := tl.poll(localSource{}, st, f); e != nil {
		t.Fatal(e)
	}
	if n := countChats(t, st); n != 1 {
		t.Fatal(n)
	}
}
func TestClockSurvivesDowntime(t *testing.T) {
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	c := eventClock{}
	c.prepare([]string{"10:00 say;1;1;A;x", "10:10 say;1;1;A;y"}, now)
	a := c.at("10:00 say;1;1;A;x", now)
	b := c.at("10:10 say;1;1;A;y", now)
	if b.Sub(a) != 10*time.Second || !b.Equal(now) {
		t.Fatalf("batch timestamps %v %v", a, b)
	}
	recovered := c.at("10:20 say;1;1;A;z", now.Add(time.Hour))
	if !recovered.Equal(now.Add(10 * time.Second)) {
		t.Fatalf("downtime became event time: %v", recovered)
	}
	if _, ok := ParseChat("12:02.5 say;1;1;A;fractional"); !ok {
		t.Fatal("fractional timestamp")
	}
}
func TestLocalSymlinkCannotEscape(t *testing.T) {
	_, _, p := recoveryFixture(t)
	outside := filepath.Join(t.TempDir(), "secret")
	os.WriteFile(outside, []byte("secret"), 0600)
	if e := os.Symlink(outside, p); e != nil {
		t.Fatal(e)
	}
	if _, e := (localSource{}).ReadAt(p, 0, 10); e == nil {
		t.Fatal("symlink escaped root")
	}
}

func TestTailRunKeepsPendingKillAcrossRestart(t *testing.T) {
	tl, st, p := recoveryFixture(t)
	tl.Loc = time.UTC
	tl.Every = 5 * time.Millisecond
	tl.Files.Games = "games_mp.log"
	p = filepath.Join(filepath.Dir(p), tl.Files.Games)
	initial := "0:00 InitGame: \\mapname\\mp_crash\\g_gametype\\sd\n0:10 KE;111;3;A;222;7;B;400;hs,ns;225;-1\n0:10 K;broken\n"
	if e := os.WriteFile(p, []byte(initial), 0600); e != nil {
		t.Fatal(e)
	}
	st.SetLogOffset(1, tl.Files.Games, 0)
	runUntil := func(want int64) {
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() { defer close(done); tl.Run(ctx, st, func([]byte) {}) }()
		deadline := time.After(3 * time.Second)
		tick := time.NewTicker(5 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-deadline:
				cancel()
				<-done
				t.Fatal("tail did not commit")
			case <-tick.C:
				off, _ := st.LogOffset(1, tl.Files.Games)
				if off >= want {
					cancel()
					<-done
					return
				}
			}
		}
	}
	runUntil(int64(len(initial)))
	var rejected int
	st.DB.QueryRow(`SELECT rejected FROM log_health WHERE file='games_mp.log'`).Scan(&rejected)
	if rejected != 1 {
		t.Fatalf("malformed event not counted: %d", rejected)
	}
	extra := "0:11 K;222;7;allies;B;111;3;axis;A;m40a3_mp;100;MOD_HEAD_SHOT;head\n"
	os.WriteFile(p, []byte(initial+extra), 0600)
	// New object: clock and KE must come from the database rather than memory.
	tl = &Tail{ServerID: 1, Dir: tl.Dir, Files: store.LogFiles{Games: "games_mp.log"}, Every: 5 * time.Millisecond, Loc: time.UTC}
	runUntil(int64(len(initial + extra)))
	var count int
	var tags string
	var distance float64
	if e := st.DB.QueryRow(`SELECT COUNT(*),tags,distance FROM kills`).Scan(&count, &tags, &distance); e != nil {
		t.Fatal(e)
	}
	if count != 1 || tags != "hs,ns" || distance != 40 {
		t.Fatalf("pending kill lost: %d %q %v", count, tags, distance)
	}
	runUntil(int64(len(initial + extra)))
	st.DB.QueryRow(`SELECT COUNT(*) FROM kills`).Scan(&count)
	if count != 1 {
		t.Fatal("duplicate after restart")
	}
}
