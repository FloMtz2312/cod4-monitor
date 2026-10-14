// Package demos records server-side demos (CoD4X "record") while enough players are on a server, keeps the list of
// the demo files, and deletes the old ones. A demo is one player's view; every real player gets one.
package demos

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"cod4monitor/internal/logtail"
	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Folder: where the demos of a server are, from its log folder (nil when the server has no logs configured).
func Folder(logURL string, s store.DemoSettings) (*url.URL, string, error) {
	if logURL == "" {
		return nil, "", nil
	}
	u, err := url.Parse(logURL)
	if err != nil {
		return nil, "", err
	}
	if strings.HasSuffix(u.Path, ".log") {
		u.Path = path.Dir(u.Path)
	}
	rel := strings.TrimSpace(s.Dir)
	if rel == "" {
		rel = store.DefaultDemoDir
	}
	dir := rel
	if !path.IsAbs(rel) {
		dir = path.Join(u.Path, rel)
	}
	dir = path.Clean(dir) + "/"
	if logtail.IsLocal(u) && !logtail.LocalOK(dir) {
		return nil, "", fmt.Errorf("dossier des démos refusé : il doit être dans %s", logtail.LocalRoot)
	}
	return u, dir, nil
}

// Status of the recording of a server, shown in its settings.
type Status struct {
	Active    bool      `json:"active"`  // demos are being recorded
	Players   int       `json:"players"` // real players at the last check
	Error     string    `json:"error,omitempty"`
	FilesErr  string    `json:"files_error,omitempty"`
	Checked   time.Time `json:"checked"`
	Scanned   time.Time `json:"scanned"`
	DiskFree  int64     `json:"disk_free,omitempty"` // bytes, when the demos are on this machine
	DiskTotal int64     `json:"disk_total,omitempty"`
}

// Recorder starts and stops the demos of one server and keeps its demo folder.
type Recorder struct {
	ServerID int64
	Name     string
	Store    *store.Store
	Rcon     func(string) (string, error) // nil: no rcon, nothing is recorded (the folder is still listed)
	LogURL   string
	Link     func(now time.Time) error // keeps the demos linked to moderation items
	Every    time.Duration

	mu     sync.Mutex
	status Status
	asked  map[string]bool // players asked to record since the recording started (slot:guid)
	below  int             // checks in a row under the threshold
	first  bool
}

func (r *Recorder) Status() Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.status
}

func (r *Recorder) log() *slog.Logger { return slog.With("server", r.Name, "part", "demos") }

// Run checks the players every Every (20 s) and the folder every minute, until ctx ends.
func (r *Recorder) Run(ctx context.Context) {
	if r.Every <= 0 {
		r.Every = 20 * time.Second
	}
	r.first = true
	t := time.NewTicker(r.Every)
	defer t.Stop()
	var scanned time.Time
	for {
		s, err := r.Store.DemoSettings(r.ServerID)
		if err != nil {
			r.log().Error("settings", "err", err)
		} else {
			r.check(s)
			// the folder is read while recording, and after, until the last demos are deleted
			if time.Since(scanned) >= time.Minute && (s.Enabled || r.hasDemos()) {
				r.scan(ctx, s, time.Now())
				scanned = time.Now()
			}
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Stop ends every demo on the server and turns the automatic recording off (option disabled).
func Stop(rcon func(string) (string, error)) error {
	if rcon == nil {
		return nil
	}
	if _, err := rcon("stoprecord all"); err != nil {
		return err
	}
	_, err := rcon("set sv_autodemorecord 0")
	return err
}

func (r *Recorder) check(s store.DemoSettings) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.Checked = time.Now()
	first := r.first
	r.first = false
	if !s.Enabled || r.Rcon == nil {
		if r.status.Active {
			r.stop()
		}
		r.status.Error = ""
		if s.Enabled {
			r.status.Error = "rcon"
		}
		return
	}
	out, err := r.Rcon("status")
	if err != nil {
		r.status.Error = err.Error()
		return
	}
	r.status.Error = ""
	var humans []query.RconPlayer
	for _, p := range query.ParseRconStatus(out) {
		if p.Addr != "bot" && !strings.HasPrefix(strings.ToLower(p.Addr), "bot") {
			humans = append(humans, p)
		}
	}
	r.status.Players = len(humans)
	if len(humans) < max(s.MinPlayers, 1) {
		// a monitor restart doesn't know what is running: make sure nothing records under the threshold
		if r.status.Active || first {
			if r.below++; r.below >= 2 || first {
				r.stop()
			}
		}
		return
	}
	r.below = 0
	if !r.status.Active {
		// players who join while it records get their demo from the server itself
		if _, err := r.Rcon("set sv_autodemorecord 1"); err != nil {
			r.status.Error = err.Error()
			return
		}
		r.status.Active, r.asked = true, map[string]bool{}
		r.log().Info("recording demos", "players", len(humans))
	}
	present := map[string]bool{}
	for _, p := range humans {
		k := fmt.Sprintf("%d:%s", p.Num, p.GUID)
		present[k] = true
		if r.asked[k] {
			continue
		}
		cmd := fmt.Sprintf("record %d", p.Num)
		if p.GUID != "" && p.GUID != "0" && isID(p.GUID) {
			cmd += " demo_" + p.GUID + "_" // the file names the player: demo_<guid>_0000.dm_1
		}
		if _, err := r.Rcon(cmd); err != nil {
			r.status.Error = err.Error()
			return
		}
		r.asked[k] = true
	}
	for k := range r.asked {
		if !present[k] {
			delete(r.asked, k)
		}
	}
}

func isID(s string) bool {
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return len(s) <= 40
}

// stop: called with r.mu held.
func (r *Recorder) stop() {
	if err := Stop(r.Rcon); err != nil {
		r.status.Error = err.Error()
		return
	}
	if r.status.Active {
		r.log().Info("demos stopped")
	}
	r.status.Active, r.asked, r.below = false, nil, 0
}

// scan lists the demo folder, forgets the files gone, keeps the linked demos and deletes the expired ones.
func (r *Recorder) scan(ctx context.Context, s store.DemoSettings, now time.Time) {
	base, dir, err := Folder(r.LogURL, s)
	if base == nil || err != nil {
		r.setFilesErr(err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	fs, err := logtail.Connect(ctx, base)
	if err != nil {
		r.setFilesErr(err)
		return
	}
	defer fs.Close()
	entries, err := fs.Entries(dir)
	if err != nil {
		r.setFilesErr(err)
		return
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if !strings.Contains(e.Name, ".dm_") {
			continue
		}
		seen[e.Name] = true
		if err := r.Store.SeeDemo(r.ServerID, e.Name, e.Size, e.Mod, now); err != nil {
			r.log().Error("demo", "err", err)
		}
	}
	known, err := r.Store.Demos(r.ServerID, store.DemoFilter{Limit: 500})
	if err != nil {
		r.log().Error("demos", "err", err)
		return
	}
	for _, d := range known {
		if !seen[d.File] {
			r.Store.DemoGone(r.ServerID, d.File, now)
		}
	}
	if r.Link != nil {
		if err := r.Link(now); err != nil {
			r.log().Warn("link demos", "err", err)
		}
	}

	var ferr error
	remove := func(ds []store.Demo) {
		for _, d := range ds {
			if now.Unix()-d.LastWrite < 120 { // still being written
				continue
			}
			if err := fs.Remove(path.Join(dir, d.File)); err != nil {
				ferr = err
				continue
			}
			r.Store.DemoGone(r.ServerID, d.File, now)
		}
	}
	expired, err := r.Store.ExpiredDemos(r.ServerID, max(s.KeepHours, 1), now)
	if err == nil {
		remove(expired)
	}
	free, total := diskSpaceOf(base, dir)
	if total > 0 && free*10 < total { // under 10 % free: the oldest normal demos go first
		if old, err := r.Store.OldestDemos(r.ServerID, 20); err == nil {
			remove(old)
			r.log().Warn("disk almost full: oldest demos deleted", "free", free)
		}
		free, total = diskSpaceOf(base, dir)
	}
	r.mu.Lock()
	r.status.Scanned, r.status.DiskFree, r.status.DiskTotal = now, free, total
	r.status.FilesErr = ""
	if ferr != nil {
		r.status.FilesErr = "suppression impossible : " + ferr.Error()
	}
	r.mu.Unlock()
}

// diskSpaceOf is diskSpace, replaced in tests.
var diskSpaceOf = diskSpace

func (r *Recorder) hasDemos() bool {
	ds, err := r.Store.Demos(r.ServerID, store.DemoFilter{Limit: 1})
	return err == nil && len(ds) > 0
}

func (r *Recorder) setFilesErr(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status.FilesErr = ""
	if err != nil {
		r.status.FilesErr = err.Error()
	}
}
