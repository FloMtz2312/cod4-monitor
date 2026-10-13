package logtail

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"cod4monitor/internal/store"
)

func TestLocalSource(t *testing.T) {
	dir := t.TempDir()
	defer func(old string) { LocalRoot = old }(LocalRoot)
	SetLocalRoot(dir)
	if err := os.Mkdir(filepath.Join(dir, "usermaps"), 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "games_mp.log")
	if err := os.WriteFile(p, []byte("line1\nline2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var s localSource
	if _, ok, err := s.Size(filepath.Join(dir, "missing.log")); ok || err != nil {
		t.Fatalf("missing file: ok=%v err=%v", ok, err)
	}
	size, ok, err := s.Size(p)
	if !ok || err != nil || size != 12 {
		t.Fatalf("size=%d ok=%v err=%v", size, ok, err)
	}
	b, err := s.ReadAt(p, 6, size-6)
	if err != nil || string(b) != "line2\n" {
		t.Fatalf("read %q err=%v", b, err)
	}
	dirs, files, err := s.List(dir)
	if err != nil || len(dirs) != 1 || dirs[0] != "usermaps" || len(files) != 1 || files[0] != "games_mp.log" {
		t.Fatalf("dirs=%v files=%v err=%v", dirs, files, err)
	}
	for in, want := range map[string]bool{"file:///cod4/mods/x/": true, "ftp://u:p@h/mods/x/": false} {
		u, _ := url.Parse(in)
		if IsLocal(u) != want {
			t.Errorf("IsLocal(%s) = %v", in, !want)
		}
	}
	if _, _, err := s.Size("/etc/passwd"); err == nil {
		t.Fatal("read outside LOCAL_LOGS_ROOT")
	}
	SetLocalRoot("/cod4")
	for in, want := range map[string]bool{"/cod4/": true, "/cod4": true, "/cod4/mods/x/": true, "/cod4/mods/x": true, "/cod4/../data/": false, "/data/": false, "cod4/mods/x/": false, "/cod4x/": false} {
		if LocalOK(in) != want {
			t.Errorf("LocalOK(%s) = %v", in, !want)
		}
	}
}

func TestProbeLocalFiles(t *testing.T) {
	root := t.TempDir()
	defer func(old string) { LocalRoot = old }(LocalRoot)
	SetLocalRoot(root)
	mod := filepath.Join(root, "mods", "m")
	if err := os.MkdirAll(filepath.Join(mod, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(mod, "logs", "games.log"), []byte("x\n"), 0o644)
	os.WriteFile(filepath.Join(root, "console.log"), []byte("x\n"), 0o644)
	dir := &url.URL{Scheme: "file", Path: mod + "/"}

	// default names: nothing there, the mod folders are suggested
	p, err := ProbeDir(context.Background(), dir, store.LogFiles{})
	if err != nil || len(p.Found) != 0 || len(p.Mods) == 0 {
		t.Fatalf("defaults: %+v %v", p, err)
	}
	// a name in the folder and an absolute path
	p, err = ProbeDir(context.Background(), dir, store.LogFiles{Games: "logs/games.log", Console: filepath.Join(root, "console.log")})
	if err != nil || len(p.Found) != 2 {
		t.Fatalf("custom: %+v %v", p, err)
	}
	tl := &Tail{Dir: dir, Files: store.LogFiles{Console: "c.log"}}
	if tl.ConsoleOld() != "c.log.old" || tl.path("/a/b.log") != "/a/b.log" || tl.path("x.log") != filepath.Join(mod, "x.log") {
		t.Fatal("paths")
	}
	// wrong folder: suggestions from the root
	p, err = ProbeDir(context.Background(), &url.URL{Scheme: "file", Path: filepath.Join(root, "nope") + "/"}, store.LogFiles{})
	if err == nil || len(p.Mods) == 0 {
		t.Fatalf("wrong folder: %+v %v", p, err)
	}
}
