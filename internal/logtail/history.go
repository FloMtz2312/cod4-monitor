package logtail

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"math"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// The history of games_mp.log: the monitor follows the log from its end the first time, so what the server wrote
// before is imported once, in the background: the players (names and GUIDs), their kills, their chat and their
// profile lines. games_mp.log only holds the time since the server started, not the date: each run of the server is
// dated with enterleave.log, whose lines carry the date, by matching the joins (J) and quits (Q) of its players with
// the connections. A run that can't be dated (enterleave.log didn't exist yet) is skipped. The rounds of that time
// aren't rebuilt: the kills have no round.

// historyState is kept in the checkpoint of "<games log>#history"; its log position is the part already imported.
type historyState struct {
	Cutoff   int64 `json:"cutoff"` // lines dated from then on were read live
	Done     bool  `json:"done"`
	Imported int   `json:"imported"`
	Undated  int   `json:"undated"`
}

const (
	historyBatch  = 5000                   // lines per transaction
	anchorWindow  = 90                     // seconds: the matches of a run agree within this
	cutoffMargin  = 120                    // seconds before the first live data: the dates of the history are estimates
	historyLineMx = 64 * 1024              // longer lines are cut
	noAnchor      = math.MinInt64          // a run that couldn't be dated
	historyWait   = 200 * time.Millisecond // between batches, to leave the database to the live log
)

// readLines calls fn with each line and the offset just after it.
func readLines(r io.Reader, fn func(line string, end int64) error) error {
	br := bufio.NewReaderSize(r, 256*1024)
	var off int64
	for {
		l, err := br.ReadString('\n')
		if len(l) > 0 {
			off += int64(len(l))
			if len(l) > historyLineMx {
				l = l[:historyLineMx]
			}
			if e := fn(strings.TrimRight(l, "\r\n"), off); e != nil {
				return e
			}
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

// presence reads a join or quit line: J;guid;slot;name or Q;guid;slot;name.
func presence(line string) (quit bool, key string, ok bool) {
	f := strings.SplitN(stampRe.ReplaceAllString(line, ""), ";", 4)
	if len(f) != 4 || (f[0] != "J" && f[0] != "Q") || f[3] == "" {
		return false, "", false
	}
	if _, err := strconv.Atoi(f[2]); err != nil {
		return false, "", false
	}
	return f[0] == "Q", f[2] + "|" + query.StripColors(f[3]), true
}

// anchors dates each run of the server in the log (unix seconds at its clock zero, noAnchor if unknown).
func anchors(r io.Reader, joins, leaves map[string][]int64) ([]int64, error) {
	var out []int64
	var cands []candidate
	last, line := -1.0, 0
	closeRun := func() {
		if last >= 0 {
			out = append(out, cluster(cands))
		}
		cands = nil
	}
	err := readLines(r, func(l string, _ int64) error {
		line++
		rel, ok := relativeSeconds(l)
		if !ok {
			return nil
		}
		if last < 0 || rel < last {
			closeRun()
		}
		last = rel
		quit, key, ok := presence(l)
		if !ok {
			return nil
		}
		times := joins[key]
		if quit {
			times = leaves[key]
		}
		for _, t := range times {
			cands = append(cands, candidate{t - int64(rel), line})
		}
		return nil
	})
	closeRun()
	return out, err
}

// candidate: the anchor a line of the log gives with one connection of enterleave.log.
type candidate struct {
	anchor int64
	line   int
}

// cluster picks the anchor most lines agree on (at least 2 lines within anchorWindow), else noAnchor.
func cluster(cands []candidate) int64 {
	slices.SortFunc(cands, func(a, b candidate) int { return cmp.Compare(a.anchor, b.anchor) })
	best, bi, bj := 0, 0, 0
	for i, j := 0, 0; j < len(cands); j++ {
		for cands[j].anchor-cands[i].anchor > anchorWindow {
			i++
		}
		if j-i+1 <= best {
			continue // fewer candidates than the best: fewer lines too
		}
		distinct := map[int]bool{}
		for _, c := range cands[i : j+1] {
			distinct[c.line] = true
		}
		if n := len(distinct); n > best {
			best, bi, bj = n, i, j
		}
	}
	if best < 2 {
		return noAnchor
	}
	return cands[(bi+bj)/2].anchor
}

// importHistory imports the history of games_mp.log once (it resumes where it stopped after a restart).
func (t *Tail) importHistory(ctx context.Context, st *store.Store) {
	games := or(t.Files.Games, "games_mp.log")
	name := games + "#history"
	log := slog.With("server_id", t.ServerID, "file", games)
	var state historyState
	if raw, err := st.LogCheckpoint(t.ServerID, name); err != nil {
		log.Error("history: read state", "err", err)
		return
	} else if raw != "" {
		json.Unmarshal([]byte(raw), &state)
	}
	if state.Done {
		return
	}
	done, err := st.LogOffset(t.ServerID, name)
	if err != nil {
		log.Error("history: read position", "err", err)
		return
	}
	if state.Cutoff == 0 {
		first, err := st.FirstGameData(t.ServerID)
		if err != nil {
			log.Error("history: first data", "err", err)
			return
		}
		state.Cutoff = time.Now().Unix()
		if first > 0 {
			state.Cutoff = min(state.Cutoff, first)
		}
		state.Cutoff -= cutoffMargin
	}
	joins, leaves, err := st.Connections(t.ServerID)
	if err != nil {
		log.Error("history: connections", "err", err)
		return
	}

	fs, err := Connect(ctx, t.Dir)
	if err != nil {
		log.Warn("history: connect", "err", err)
		return
	}
	defer fs.Close()
	open := func() (io.ReadCloser, error) { return fs.Open(t.path(games)) }
	// over FTP the whole file is downloaded twice: not for a big file (HISTORY_FTP_MAX_MB)
	if !IsLocal(t.Dir) && t.HistoryFTPMax != 0 {
		var size int64 = -1
		if es, err := fs.Entries(path.Dir(t.path(games))); err == nil {
			for _, e := range es {
				if e.Name == path.Base(t.path(games)) {
					size = e.Size
				}
			}
		}
		if t.HistoryFTPMax < 0 || size > t.HistoryFTPMax {
			log.Info("history: not imported over FTP", "size", size, "max", t.HistoryFTPMax)
			state.Done = true
			raw, _ := json.Marshal(state)
			if err := st.SetCheckpoint(t.ServerID, name, string(raw)); err != nil {
				log.Error("history: save", "err", err)
			}
			return
		}
	}
	rd, err := open()
	if err != nil {
		log.Warn("history: open", "err", err)
		return
	}
	runs, err := anchors(rd, joins, leaves)
	rd.Close()
	if err != nil {
		log.Warn("history: read", "err", err)
		return
	}
	dated := 0
	for _, a := range runs {
		if a != noAnchor {
			dated++
		}
	}
	log.Info("history: importing games_mp.log written before the monitor", "runs", len(runs), "dated", dated, "cutoff", time.Unix(state.Cutoff, 0))

	if rd, err = open(); err != nil {
		log.Warn("history: open", "err", err)
		return
	}
	defer rd.Close()
	var batch []string
	var stamps []int64
	var end int64
	save := func(final bool) error {
		state.Done = final
		raw, _ := json.Marshal(state)
		lines, ts := batch, stamps
		err := st.Ingest(t.ServerID, name, end, string(raw), func(b *store.Store) {
			for i, l := range lines {
				t.pastLine(b, l, time.Unix(ts[i], 0))
			}
		})
		batch, stamps = batch[:0], stamps[:0]
		if err == nil && !final {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(historyWait):
			}
		}
		return err
	}
	run, last := -1, -1.0
	errStop := io.EOF
	err = readLines(rd, func(l string, off int64) error {
		rel, ok := relativeSeconds(l)
		if !ok {
			return nil
		}
		if last < 0 || rel < last {
			run++
		}
		last = rel
		if off <= done || run >= len(runs) {
			return nil
		}
		if runs[run] == noAnchor {
			if _, _, ok := ParseIdentity(l); ok {
				state.Undated++
			}
			return nil
		}
		ts := runs[run] + int64(rel)
		if ts >= state.Cutoff {
			return errStop // the rest was read live
		}
		end = off
		batch, stamps = append(batch, l), append(stamps, ts)
		state.Imported++
		if len(batch) >= historyBatch {
			return save(false)
		}
		return nil
	})
	if err != nil && err != errStop {
		log.Warn("history: stopped, resumes at the next start", "err", err)
		return
	}
	if err := save(true); err != nil {
		log.Error("history: save", "err", err)
		return
	}
	log.Info("history: done", "lines", state.Imported, "undated_player_lines", state.Undated)
}

// pastLine stores a line of the history: players, kills, chat, profiles (no rounds, no live side effects).
func (t *Tail) pastLine(st *store.Store, l string, ts time.Time) {
	if _, err := storePast(st, t.ServerID, l, ts, false); err != nil {
		slog.Error("history: store line", "server_id", t.ServerID, "err", err)
	}
}

// pastWindow: an imported event within this many seconds of a stored one with the same players is the same event.
const pastWindow = 180

// storePast stores a past line of games_mp.log. With dedup, a kill, chat message or profile already stored (read
// live, or imported before) is skipped: stored=false.
func storePast(st *store.Store, serverID int64, l string, ts time.Time, dedup bool) (stored bool, err error) {
	known := func(kind, a, b, c string) bool {
		if !dedup || err != nil {
			return false
		}
		var k bool
		k, err = st.PastEventKnown(serverID, kind, ts.Unix(), pastWindow, a, b, c)
		return k
	}
	if guid, name, ok := ParseIdentity(l); ok {
		if err = st.SeenName(serverID, ts, guid, name, query.StripColors(name)); err != nil {
			return false, err
		}
		stored = true
	}
	if c, ok := ParseChat(l); ok {
		if known("chat", c.NameClean, c.Message, "") {
			return false, err
		}
		return true, st.AddChat(serverID, ts, c.GUID, c.NameRaw, c.NameClean, c.Message, c.Team)
	}
	if k, ok := ParseKill(l); ok {
		killer := ""
		if k.KillerName != "" {
			killer = store.PlayerKey(k.KillerGUID, query.StripColors(k.KillerName))
		}
		if known("kill", killer, store.PlayerKey(k.VictimGUID, query.StripColors(k.VictimName)), k.Weapon) {
			return false, err
		}
		return true, st.AddPastKill(serverID, ts, k.KillerGUID, k.KillerName, query.StripColors(k.KillerName), k.KillerTeam,
			k.VictimGUID, k.VictimName, query.StripColors(k.VictimName), k.VictimTeam, k.Weapon, k.Cause, k.HitLoc, k.Damage)
	}
	if p, ok := ParseProfile(l); ok {
		if known("profile", store.PlayerKey(p.GUID, query.StripColors(p.Name)), "", "") {
			return false, err
		}
		n := p.N
		return true, st.AddProfile(serverID, ts, p.GUID, p.Name, query.StripColors(p.Name), p.PlayerID, n[0], n[1], n[2], n[3], n[4], n[5])
	}
	return stored, err
}
