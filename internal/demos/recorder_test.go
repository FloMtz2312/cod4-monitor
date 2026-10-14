package demos

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/logtail"
	"cod4monitor/internal/store"
)

const statusOut = `map: mp_crash
num score ping playerid steamid name lastmsg address qport rate
--- ----- ---- ---------- ------- ---------------- ------- --------------------- ----- -----
  0    10   40 2310346616543210987 0 Kestrel^7 0 203.0.113.5:28960 1234 25000
  1     0    0 0 0 Bot0^7 0 bot 0 25000
  2     5   55 2310346616543210988 0 Nitro^7 0 203.0.113.6:28960 4321 25000
`

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "m.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.DB.Close() })
	if _, err := st.UpsertServer("Test", "203.0.113.10", 28960); err != nil {
		t.Fatal(err)
	}
	return st
}

type fakeRcon struct {
	out  string
	sent []string
}

func (f *fakeRcon) run(cmd string) (string, error) {
	f.sent = append(f.sent, cmd)
	if cmd == "status" {
		return f.out, nil
	}
	return "", nil
}

func (f *fakeRcon) commands() string {
	var out []string
	for _, c := range f.sent {
		if c != "status" {
			out = append(out, c)
		}
	}
	f.sent = nil
	return strings.Join(out, " | ")
}

func TestRecorder(t *testing.T) {
	st := newStore(t)
	rc := &fakeRcon{out: statusOut}
	r := &Recorder{ServerID: 1, Name: "Test", Store: st, Rcon: rc.run}

	// disabled: nothing is sent
	r.check(store.DemoSettings{Enabled: false, MinPlayers: 2, KeepHours: 48})
	if got := rc.commands(); got != "" {
		t.Fatalf("disabled sent %q", got)
	}
	// enabled with 2 real players (the bot doesn't count, and isn't recorded)
	s := store.DemoSettings{Enabled: true, MinPlayers: 2, KeepHours: 48}
	r.first = false
	r.check(s)
	want := "set sv_autodemorecord 1 | record 0 demo_2310346616543210987_ | record 2 demo_2310346616543210988_"
	if got := rc.commands(); got != want {
		t.Fatalf("start:\n got %q\nwant %q", got, want)
	}
	if st := r.Status(); !st.Active || st.Players != 2 {
		t.Fatalf("status %+v", st)
	}
	// next check: nobody asked twice
	r.check(s)
	if got := rc.commands(); got != "" {
		t.Fatalf("second check sent %q", got)
	}
	// a player leaves: under the threshold twice in a row before stopping
	rc.out = strings.Replace(statusOut, "  2     5   55 2310346616543210988 0 Nitro^7 0 203.0.113.6:28960 4321 25000\n", "", 1)
	r.check(s)
	if got := rc.commands(); got != "" {
		t.Fatalf("stopped too early: %q", got)
	}
	r.check(s)
	if got := rc.commands(); got != "stoprecord all | set sv_autodemorecord 0" {
		t.Fatalf("stop: %q", got)
	}
	// disabled while recording: stops
	rc.out = statusOut
	r.check(s)
	rc.commands()
	r.check(store.DemoSettings{Enabled: false})
	if got := rc.commands(); got != "stoprecord all | set sv_autodemorecord 0" {
		t.Fatalf("disable: %q", got)
	}
}

func TestScanKeepDelete(t *testing.T) {
	root := t.TempDir()
	logtail.SetLocalRoot(root)
	t.Cleanup(func() { logtail.LocalRoot = "/gamelogs/" })
	logs := filepath.Join(root, "mods", "mymod")
	demosDir := filepath.Join(root, "demos")
	for _, d := range []string{logs, demosDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	write := func(name string, age time.Duration) {
		p := filepath.Join(demosDir, name)
		os.WriteFile(p, make([]byte, 1000), 0o644)
		os.Chtimes(p, now.Add(-age), now.Add(-age))
	}
	write("demo_111_0000.dm_1", 72*time.Hour) // old: deleted
	write("demo_222_0000.dm_1", 72*time.Hour) // old but linked to a report: kept
	write("demo_333_0000.dm_1", time.Hour)    // recent: kept
	write("notes.txt", 72*time.Hour)          // not a demo: never touched

	diskSpaceOf = func(*url.URL, string) (int64, int64) { return 50e9, 100e9 }
	t.Cleanup(func() { diskSpaceOf = diskSpace })
	st := newStore(t)
	r := &Recorder{ServerID: 1, Name: "Test", Store: st, LogURL: "file://" + logs + "/",
		Link: func(time.Time) error {
			_, err := st.LinkDemos(1, "222", now.Add(-72*time.Hour).Unix(), "report")
			return err
		}}
	r.scan(t.Context(), store.DemoSettings{Enabled: true, MinPlayers: 2, KeepHours: 48}, now)
	if e := r.Status().FilesErr; e != "" {
		t.Fatal(e)
	}
	for name, want := range map[string]bool{"demo_111_0000.dm_1": false, "demo_222_0000.dm_1": true, "demo_333_0000.dm_1": true, "notes.txt": true} {
		if _, err := os.Stat(filepath.Join(demosDir, name)); (err == nil) != want {
			t.Errorf("%s exists=%v, want %v", name, err == nil, want)
		}
	}
	ds, err := st.Demos(1, store.DemoFilter{})
	if err != nil || len(ds) != 2 {
		t.Fatalf("demos %v %+v", err, ds)
	}
	for _, d := range ds {
		if d.Player == "222" && (d.Kept != store.DemoLinked || d.KeptReason != "report") {
			t.Errorf("linked demo %+v", d)
		}
	}
	// a demo folder outside the allowed local root is refused
	if _, _, err := Folder("file://"+logs+"/", store.DemoSettings{Dir: "/etc/"}); err == nil {
		t.Error("folder outside LocalRoot accepted")
	}
}
