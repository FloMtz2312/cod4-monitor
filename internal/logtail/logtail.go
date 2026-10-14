// Package logtail follows a CoD4X server's log files, over FTP or in a local folder:
//   - qconsole.log:   streamed live to the page, kept in memory only
//   - games_mp.log:   chat messages and GUID <-> name, stored
//   - enterleave.log: connections with real timestamps, IP, steamid and GUID, stored
package logtail

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

type Chat struct {
	GUID, NameRaw, NameClean, Message string
	Team                              bool
	Slot                              int
}

var (
	stampRe  = regexp.MustCompile(`^\s*\d+:\d+(?:\.\d+)?\s+`)
	errorRe  = regexp.MustCompile(`(?i)\berror\b|\bwarning\b`)
	digitsRe = regexp.MustCompile(`\d+`) // "Waited 117 msec" and "Waited 98 msec" are the same problem
)

// ParseChat recognises `say;GUID;slot;name;message` and `sayteam;...` lines.
func ParseChat(line string) (Chat, bool) {
	f := strings.SplitN(stampRe.ReplaceAllString(line, ""), ";", 5)
	if len(f) != 5 || (f[0] != "say" && f[0] != "sayteam") {
		return Chat{}, false
	}
	msg := strings.Map(func(r rune) rune {
		if r < 0x20 { // CoD prefixes chat with \x15
			return -1
		}
		return r
	}, f[4])
	slot, _ := strconv.Atoi(f[2])
	return Chat{GUID: f[1], NameRaw: f[3], NameClean: query.StripColors(f[3]), Message: msg, Team: f[0] == "sayteam", Slot: slot}, true
}

// ParseIdentity extracts GUID and name from join/quit/chat lines (`J;GUID;slot;name`, `say;GUID;slot;name;msg`...).
// GUID "0" (bots, unauthenticated players) is ignored.
func ParseIdentity(line string) (guid, name string, ok bool) {
	f := strings.SplitN(stampRe.ReplaceAllString(line, ""), ";", 5)
	if len(f) < 4 || (f[0] != "J" && f[0] != "Q" && f[0] != "say" && f[0] != "sayteam") || f[1] == "" || f[1] == "0" || f[3] == "" {
		return "", "", false
	}
	return f[1], f[3], true
}

type Kill struct {
	VictimSlot                         int
	VictimGUID, VictimName, VictimTeam string
	KillerGUID, KillerName, KillerTeam string // empty killer: suicide or world (fall...)
	Weapon, Cause, HitLoc              string
	Damage                             int
}

// ParseKill reads `K;vGUID;vSlot;vTeam;vName;aGUID;aSlot;aTeam;aName;weapon;damage;MOD;hitloc`.
func ParseKill(line string) (Kill, bool) {
	f := strings.Split(stampRe.ReplaceAllString(line, ""), ";")
	if len(f) != 13 || f[0] != "K" {
		return Kill{}, false
	}
	k := Kill{VictimGUID: f[1], VictimTeam: f[3], VictimName: f[4], Weapon: f[9], Cause: f[11], HitLoc: f[12]}
	var e1, e2 error
	k.VictimSlot, e1 = strconv.Atoi(f[2])
	k.Damage, e2 = strconv.Atoi(f[10])
	if e1 != nil || e2 != nil || k.VictimSlot < 0 || k.Damage < 0 {
		return Kill{}, false
	}
	if f[6] != "-1" && f[5] != f[1] && f[8] != "" { // a real attacker
		k.KillerGUID, k.KillerTeam, k.KillerName = f[5], f[7], f[8]
	}
	return k, true
}

// ParseInitGame returns map and gametype of an `InitGame: \key\value...` line (new round).
func ParseInitGame(line string) (mapName, gametype string, ok bool) {
	rest, ok := strings.CutPrefix(stampRe.ReplaceAllString(line, ""), "InitGame: ")
	if !ok {
		return "", "", false
	}
	kv := strings.Split(strings.TrimPrefix(rest, `\`), `\`)
	for i := 0; i+1 < len(kv); i += 2 {
		switch kv[i] {
		case "mapname":
			mapName = kv[i+1]
		case "g_gametype":
			gametype = kv[i+1]
		}
	}
	return mapName, gametype, true
}

// Accuracy is a P_A line: P_A;guid;slot;name;shots;hits;damage done;damage taken;friendly done;friendly taken
type Accuracy struct {
	GUID, Name string
	N          [6]int // shots, hits, damage done, damage taken, friendly damage done, friendly damage taken
}

func ParseAccuracy(line string) (Accuracy, bool) {
	f := strings.Split(stampRe.ReplaceAllString(line, ""), ";")
	if len(f) != 10 || f[0] != "P_A" {
		return Accuracy{}, false
	}
	a := Accuracy{GUID: f[1], Name: f[3]}
	for i := range a.N {
		var err error
		if a.N[i], err = strconv.Atoi(f[4+i]); err != nil || a.N[i] < 0 {
			return Accuracy{}, false
		}
	}
	return a, true
}

type EnterLeave struct {
	TS                time.Time
	Enter             bool
	NameRaw, Addr     string // Addr is ip:port
	Slot              int
	SteamID, PlayerID string // PlayerID is the GUID on leave lines, 0 or the steamid on enter lines
}

var elRe = regexp.MustCompile(`^(\w{3} \w{3} +\d+ \d\d:\d\d:\d\d \d{4}): \^\dClient (.*) (\S+) \^\d(entered this server in|left this server from) slot (\d+) with (?:steamid (\d+) )?playerid (\d+)$`)

// ParseEnterLeave parses an enterleave.log line; bots are skipped. Times are in the server's zone loc.
func ParseEnterLeave(line string, loc *time.Location) (EnterLeave, bool) {
	m := elRe.FindStringSubmatch(line)
	if m == nil || m[3] == "bot" {
		return EnterLeave{}, false
	}
	ts, err := time.ParseInLocation("Mon Jan _2 15:04:05 2006", m[1], loc)
	if err != nil {
		return EnterLeave{}, false
	}
	slot, _ := strconv.Atoi(m[5])
	return EnterLeave{TS: ts, Enter: strings.HasPrefix(m[4], "entered"), NameRaw: m[2], Addr: m[3], Slot: slot, SteamID: m[6], PlayerID: m[7]}, true
}

// Tail keeps the last console lines in memory for the page and pushes new lines to publish.
type Tail struct {
	ServerID int64
	Dir      *url.URL       // ftp://user:pass@host:21/mods/<mod>/ or file:///<folder>/ — folder holding the logs
	Files    store.LogFiles // names or paths of the logs, empty = default
	Every    time.Duration
	Loc      *time.Location                   // server clock, for enterleave.log
	Rcon     func(cmd string) (string, error) // nil without rcon: no answer to chat commands, no welcome message
	OnJoin   func(guid, nameRaw string)       // a player connects (live lines only): player alerts by mail
	SiteURL  string                           // public address of the dashboard (DOMAIN), for the !me link

	mu      sync.Mutex
	last    []string
	lastCmd map[string]time.Time // chat command cooldowns per slot
	clock   eventClock
	pending *KillEvent // KE line waiting for its K line
	history bool       // the import of the history of games_mp.log was started
}

const keep = 500

func (t *Tail) Recent() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([]string{}, t.last...) // non-nil: "log configured, nothing yet"
}

// file is one followed log. persist=false: start near the end on every (re)connect.
type file struct {
	name       string
	persist    bool
	fromStart  bool                                                // first run ever: import the whole file (enterleave has real timestamps)
	handle     func(st *store.Store, lines []string, backlog bool) // backlog: first read of a non-persisted file
	off        int64
	partial    string
	loaded     bool
	missing    bool
	checkpoint checkpoint
	catching   bool
}

// Run reconnects forever until ctx is cancelled.
func (t *Tail) Run(ctx context.Context, st *store.Store, publish func([]byte)) {
	log := slog.With("server_id", t.ServerID, "dir", t.Dir.Redacted())
	emit := func(ev map[string]any) {
		ev["server_id"] = t.ServerID
		b, _ := json.Marshal(ev)
		publish(b)
	}
	var activeBatch *store.Store
	rawEmit := emit
	emit = func(ev map[string]any) {
		if activeBatch != nil {
			activeBatch.AfterCommit(func(_ *store.Store) { rawEmit(ev) })
		} else {
			rawEmit(ev)
		}
	}
	files := []*file{
		{name: or(t.Files.Console, "qconsole.log"), handle: func(st *store.Store, lines []string, backlog bool) {
			for i, l := range lines {
				lines[i] = query.RedactSecrets(query.LocalizeDates(l, t.Loc, time.Local))
				if !backlog && errorRe.MatchString(l) {
					if err := st.ConsoleError(t.ServerID, time.Now(), query.RedactSecrets(query.StripColors(digitsRe.ReplaceAllString(l, "#")))); err != nil {
						log.Error("store console error", "err", err)
					}
				}
			}
			t.mu.Lock()
			t.last = append(t.last, lines...)
			if len(t.last) > keep {
				t.last = t.last[len(t.last)-keep:]
			}
			t.mu.Unlock()
			for _, l := range lines {
				emit(map[string]any{"type": "log", "line": l})
			}
		}},
		{name: or(t.Files.Games, "games_mp.log"), persist: true, handle: func(st *store.Store, lines []string, catchUp bool) {
			// Retain the game clock anchor across batches and monitor restarts.
			t.clock.prepare(lines, time.Now())
			for _, l := range lines {
				now := t.clock.at(l, time.Now())
				if t.gameEvent(st, l, now, catchUp, rawEmit) {
					continue
				}
				if guid, name, ok := ParseIdentity(l); ok {
					if err := st.SeenName(t.ServerID, now, guid, name, query.StripColors(name)); err != nil {
						log.Error("store name", "err", err)
					}
				}
				if a, ok := ParseAccuracy(l); ok {
					n := a.N
					if err := st.AddAccuracy(t.ServerID, now, a.GUID, a.Name, query.StripColors(a.Name), n[0], n[1], n[2], n[3], n[4], n[5]); err != nil {
						log.Error("store accuracy", "err", err)
					}
					continue
				}
				if k, ok := ParseKill(l); ok {
					err := st.AddKill(t.ServerID, now, k.KillerGUID, k.KillerName, query.StripColors(k.KillerName), k.KillerTeam,
						k.VictimGUID, k.VictimName, query.StripColors(k.VictimName), k.VictimTeam, k.Weapon, k.Cause, k.HitLoc, k.Damage)
					if err != nil {
						log.Error("store kill", "err", err)
					}
					if ke := t.pending; ke != nil && err == nil && ke.VictimSlot == k.VictimSlot && ke.KillerName == k.KillerName {
						err = st.SetLastKillExtra(t.ServerID, store.KillExtra{Distance: ke.Distance, Tags: ke.Tags, Points: ke.Points, AdsMs: ke.AdsMs})
						if err != nil {
							log.Error("store kill details", "err", err)
						}
					}
					t.pending = nil
					continue
				}
				if m, gt, ok := ParseInitGame(l); ok {
					if err := st.StartRound(t.ServerID, now, m, gt); err != nil {
						log.Error("store round", "err", err)
					}
					continue
				}
				if strings.Contains(l, "ExitLevel: executed") || strings.Contains(l, "ShutdownGame:") {
					if err := st.EndRound(t.ServerID, now); err != nil {
						log.Error("store round", "err", err)
					}
					continue
				}
				if c, ok := ParseChat(l); ok {
					if err := st.AddChat(t.ServerID, now, c.GUID, c.NameRaw, c.NameClean, c.Message, c.Team); err != nil {
						log.Error("store chat", "err", err)
					}
					emit(map[string]any{"type": "chat", "chat": map[string]any{"name_raw": c.NameRaw, "message": c.Message, "team": c.Team, "ts": now.Unix()}})
					if !catchUp && strings.HasPrefix(c.Message, "!") && c.GUID != "" {
						st.AfterCommit(func(root *store.Store) { go t.command(root, c, rawEmit) })
					}
				}
			}
		}},
		{name: or(t.Files.EnterLeave, "enterleave.log"), persist: true, fromStart: true, handle: func(st *store.Store, lines []string, _ bool) {
			for _, l := range lines {
				e, ok := ParseEnterLeave(l, t.Loc)
				if !ok {
					continue
				}
				ip := query.HostOf(e.Addr)
				var err error
				if e.Enter {
					err = st.Enter(t.ServerID, e.TS, e.Slot, e.Addr, ip, e.SteamID, e.NameRaw, query.StripColors(e.NameRaw))
				} else {
					err = st.Leave(t.ServerID, e.TS, e.Slot, e.Addr, e.PlayerID, e.NameRaw, query.StripColors(e.NameRaw))
				}
				if err != nil {
					log.Error("store connection", "err", err)
				}
				emit(map[string]any{"type": "connection", "enter": e.Enter, "name_raw": e.NameRaw})
			}
		}},
	}
	for _, f := range files {
		original := f.handle
		f.handle = func(batch *store.Store, lines []string, backlog bool) {
			activeBatch = batch
			defer func() { activeBatch = nil }()
			original(batch, lines, backlog)
		}
	}
	for {
		err := t.follow(ctx, st, files)
		if ctx.Err() != nil {
			return
		}
		t.recordError(st, "connection")
		log.Error("log tail failed, retrying in 15s", "err", err)
		select {
		case <-ctx.Done():
			return
		case <-time.After(15 * time.Second):
		}
	}
}

// gameEvent stores the lines of the pm_sniper mod (KE, RE, CL, MS, ME, MV) and the objective lines; true if the line was one.
func (t *Tail) gameEvent(st *store.Store, l string, now time.Time, catchUp bool, emit func(map[string]any)) bool {
	var err error
	if ke, ok := ParseKillEvent(l); ok {
		t.pending = &ke
		return true
	}
	if re, ok := ParseRoundEnd(l); ok {
		err = st.FinishRound(t.ServerID, now, re.Winner, re.Allies, re.Axis, re.Attackers)
	} else if e, ok := ParsePlayerEvent(l); ok {
		err = st.AddGameEvent(t.ServerID, now, e.Type, e.GUID, e.Name, query.StripColors(e.Name), e.Value)
	} else if m, ok := ParseMatchScore(l); ok {
		err = st.AddMatchScore(t.ServerID, now, m.GUID, m.Name, query.StripColors(m.Name), m.Team, m.N[0], m.N[1], m.N[2], m.N[3], m.N[4])
	} else if w, a, x, ok := ParseMatchEnd(l); ok {
		err = st.AddMatchResult(t.ServerID, now, w, a, x)
	} else if mp, mode, votes, chosen, ok := ParseMapVote(l); ok {
		err = st.AddMapVote(t.ServerID, now, mp, mode, votes, chosen)
	} else if p, ok := ParseProfile(l); ok {
		n := p.N
		err = st.AddProfile(t.ServerID, now, p.GUID, p.Name, query.StripColors(p.Name), p.PlayerID, n[0], n[1], n[2], n[3], n[4], n[5])
		if err == nil && !catchUp {
			st.AfterCommit(func(_ *store.Store) { emit(map[string]any{"type": "profile"}) }) // the dashboard re-checks the ban evasion clues
		}
	} else {
		if guid, slot, name, ok := ParseJoin(l); ok && !catchUp {
			st.AfterCommit(func(root *store.Store) {
				go t.welcome(root, guid, slot, name)
				go t.fingerprint(root, guid, slot, name, emit)
				if t.OnJoin != nil {
					go t.OnJoin(guid, name)
				}
			})
		}
		return false // J lines also give the GUID <-> name link
	}
	if err != nil {
		slog.Error("store game event", "server_id", t.ServerID, "line", l, "err", err)
	}
	return true
}

// GameDirs lists the folders of mods/ and usermaps/ on the server (the logs live in mods/<mod>/).
func (t *Tail) GameDirs(ctx context.Context) (mods, usermaps []string, err error) {
	c, err := t.connect(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer c.Close()
	root := path.Dir(path.Dir(path.Clean(t.Dir.Path)))
	list := func(dir string) ([]string, error) {
		dirs, _, err := c.List(path.Join(root, dir))
		return dirs, err
	}
	if mods, err = list("mods"); err != nil {
		return nil, nil, err
	}
	usermaps, err = list("usermaps")
	return mods, usermaps, err
}

func (t *Tail) follow(ctx context.Context, st *store.Store, files []*file) error {
	c, err := t.connect(ctx)
	if err != nil {
		return err
	}
	defer c.Close()
	slog.Info("log tail connected", "server_id", t.ServerID, "dir", t.Dir.Path)

	tick := time.NewTicker(t.Every)
	defer tick.Stop()
	for {
		caughtUp := true
		for _, f := range files {
			if err := t.poll(c, st, f); err != nil {
				return err
			}
			caughtUp = caughtUp && (f.missing || f.loaded && !f.catching)
		}
		// once enterleave.log is read (it dates the history), the part of games_mp.log written before the monitor
		if caughtUp && !t.history {
			t.history = true
			go t.importHistory(ctx, st)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-tick.C:
		}
	}
}

func or(s, def string) string {
	if s = strings.TrimSpace(s); s != "" {
		return s
	}
	return def
}

// path of a log: a name in the log folder, or an absolute path.
func (t *Tail) path(name string) string {
	if path.IsAbs(name) {
		return path.Clean(name)
	}
	return path.Join(t.Dir.Path, name)
}

// ConsoleOld is the console of the previous run: CoD4X renames the console log to <name>.old when it starts.
func (t *Tail) ConsoleOld() string { return or(t.Files.Console, "qconsole.log") + ".old" }

const maxRead = 256 * 1024
const maxLine = 64 * 1024

type checkpoint struct {
	Fingerprint string     `json:"fingerprint"`
	Clock       eventClock `json:"clock"`
	Pending     *KillEvent `json:"pending,omitempty"`
}

func fingerprint(c source, p string, off int64) (string, error) {
	if off == 0 {
		return "", nil
	}
	b, err := c.ReadAt(p, max(0, off-128), min(off, 128))
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), nil
}

func (t *Tail) poll(c source, st *store.Store, f *file) (retErr error) {
	defer func() {
		if retErr != nil {
			t.recordError(st, f.name)
			f.loaded = false
			f.partial = ""
		}
	}()
	p := t.path(f.name)
	size, ok, err := c.Size(p)
	if err != nil {
		return err
	}
	if !ok {
		t.recordMissing(st, f.name)
		f.missing = true
		return nil
	}
	f.missing = false
	first := !f.loaded
	skipFirst := false
	if first {
		f.off = -1
		if f.persist {
			f.off, err = st.LogOffset(t.ServerID, f.name)
			if err != nil {
				return err
			}
			state, e := st.LogCheckpoint(t.ServerID, f.name)
			if e != nil {
				return e
			}
			if state != "" {
				if e = json.Unmarshal([]byte(state), &f.checkpoint); e != nil {
					return e
				}
			}
		}
		switch {
		case f.off >= 0:
			f.catching = true
		case f.fromStart:
			f.off = 0
			f.catching = true
		case f.persist:
			f.off = size
		default:
			f.off = max(0, size-64*1024)
			skipFirst = f.off > 0
			f.catching = true
		}
		f.loaded = true
	}
	committed := f.off - int64(len(f.partial))
	if size < f.off {
		f.off = 0
		f.partial = ""
		f.checkpoint = checkpoint{}
		t.recordRotation(st, f.name)
	} else if f.checkpoint.Fingerprint != "" {
		got, e := fingerprint(c, p, committed)
		if e != nil {
			return e
		}
		if got != f.checkpoint.Fingerprint {
			f.off = 0
			f.partial = ""
			f.checkpoint = checkpoint{}
			t.recordRotation(st, f.name)
		}
	}
	if f.name == or(t.Files.Games, "games_mp.log") {
		t.clock = f.checkpoint.Clock
		t.pending = f.checkpoint.Pending
	}
	oldOff, oldPartial, oldCheckpoint := f.off, f.partial, f.checkpoint
	b, err := c.ReadAt(p, f.off, min(size-f.off, maxRead))
	if err != nil {
		return err
	}
	nextOff := f.off + int64(len(b))
	lines := strings.Split(f.partial+string(b), "\n")
	partial := lines[len(lines)-1]
	lines = lines[:len(lines)-1]
	if skipFirst && len(lines) > 0 {
		lines = lines[1:]
	}
	out := make([]string, 0, len(lines))
	rejected := 0
	for _, l := range lines {
		l = strings.TrimRight(l, "\r")
		if len(l) > maxLine {
			rejected++
			continue
		}
		if l != "" {
			if f.name == or(t.Files.Games, "games_mp.log") && malformedGameLine(l) {
				rejected++
				continue
			}
			out = append(out, l)
		}
	}
	// Do not checkpoint a fragment: even oversized input remains restart-safe.
	if len(partial) > maxLine {
		return fmt.Errorf("log line exceeds %d bytes", maxLine)
	}
	nextCommitted := nextOff - int64(len(partial))
	fp, err := fingerprint(c, p, nextCommitted)
	if err != nil {
		return err
	}
	// Detect a replacement while the range was being read; retry without committing any events.
	if len(b) > 0 && nextCommitted > oldOff {
		tail := append([]byte(oldPartial), b...)
		n := min(int64(128), nextCommitted-(oldOff-int64(len(oldPartial))))
		if n > 0 {
			check, e := c.ReadAt(p, nextCommitted-n, n)
			if e != nil {
				return e
			}
			end := len(tail) - len(partial)
			if !bytes.Equal(check, tail[end-int(n):end]) {
				return fmt.Errorf("log changed during read")
			}
		}
	}
	f.off, f.partial = nextOff, partial
	handle := func(batch *store.Store) {
		f.handle(batch, out, f.catching)
		t.recordRead(batch, f.name, size-nextCommitted, len(out), rejected)
	}
	if f.persist {
		// Clock and pending KE are advanced inside the same database transaction as the cursor.
		err = st.Ingest(t.ServerID, f.name, nextCommitted, "", func(batch *store.Store) {
			handle(batch)
			cp := checkpoint{Fingerprint: fp, Clock: t.clock, Pending: t.pending}
			raw, _ := json.Marshal(cp)
			batch.SetCheckpoint(t.ServerID, f.name, string(raw))
			f.checkpoint = cp
		})
	} else {
		handle(st)
		f.checkpoint.Fingerprint = fp
	}
	if err != nil {
		f.off, f.partial, f.checkpoint = oldOff, oldPartial, oldCheckpoint
		return err
	}
	if nextOff >= size {
		f.catching = false
	}
	return nil
}

// ReadEnd returns the last lines of a file of the log folder (up to 64 KB), e.g. qconsole.log.old: CoD4X renames
// the console log to that name when it starts, so after a restart it holds the end of the previous run.
func (t *Tail) ReadEnd(ctx context.Context, name string) ([]string, error) {
	c, err := t.connect(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	p := t.path(name)
	size, ok, err := c.Size(p)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, errors.New(name + " introuvable")
	}
	off := max(0, size-64*1024)
	b, err := c.ReadAt(p, off, size-off)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	if off > 0 && len(lines) > 0 {
		lines = lines[1:] // probably mid-line
	}
	out := lines[:0]
	for _, l := range lines {
		if l = strings.TrimRight(l, "\r"); l != "" {
			out = append(out, query.RedactSecrets(query.LocalizeDates(l, t.Loc, time.Local)))
		}
	}
	return out, nil
}

// Probe checks a log folder (FTP or local) before it is saved: it logs in and tells which of the three logs are there.
// When none is, Mods lists folders the log folder could be, to help find the right one.
type Probe struct {
	Found []string `json:"found"`          // log files present in the folder
	Mods  []string `json:"mods,omitempty"` // folders the folder could be
}

func ProbeDir(ctx context.Context, dir *url.URL, files store.LogFiles) (Probe, error) {
	t := &Tail{Dir: dir, Files: files}
	var p Probe
	c, err := t.connect(ctx)
	if err != nil {
		if IsLocal(dir) { // wrong folder: suggest the ones there are
			p.Mods = suggest(localSource{}, []string{LocalRoot, path.Join(LocalRoot, "mods")}, true)
		}
		return p, err
	}
	defer c.Close()
	for _, name := range []string{or(files.Games, "games_mp.log"), or(files.Console, "qconsole.log"), or(files.EnterLeave, "enterleave.log")} {
		_, ok, err := c.Size(t.path(name))
		if err != nil && !IsLocal(dir) {
			return p, err
		}
		if ok {
			p.Found = append(p.Found, name)
		}
	}
	if len(p.Found) == 0 {
		roots := []string{"/mods", "mods", "/"}
		if IsLocal(dir) {
			roots = []string{LocalRoot, path.Join(LocalRoot, "mods")}
		}
		p.Mods = suggest(c, roots, IsLocal(dir))
	}
	return p, nil
}

// suggest lists the sub-folders of the roots: of all of them, or of the first that has some.
func suggest(c source, roots []string, all bool) []string {
	var out []string
	for _, root := range roots {
		if ds, _, err := c.List(root); err == nil && len(ds) > 0 {
			for _, d := range ds {
				out = append(out, path.Join(root, d)+"/")
			}
			if !all {
				break
			}
		}
	}
	return out
}
