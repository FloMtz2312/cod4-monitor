package logtail

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"cod4monitor/internal/query"
	"cod4monitor/internal/store"
)

// Old log files uploaded from the dashboard (the file a server left aside, a backup…), imported in the background
// next to the live reading. What is already stored is skipped, so a file can be imported again safely.

// ImportProgress is the state of an import, reported as it goes.
type ImportProgress struct {
	Kind                              string // "enterleave" or "games"
	Lines, Imported, Skipped, Undated int64
}

// ErrUnknownLog: the file is neither an enterleave.log nor a games_mp.log.
var ErrUnknownLog = errors.New("ni un enterleave.log ni un games_mp.log")

// DetectKind reads the start of a log file: "enterleave", "games" or "".
func DetectKind(r io.Reader, loc *time.Location) string {
	sc := bufio.NewScanner(io.LimitReader(r, 4<<20))
	sc.Buffer(make([]byte, 64*1024), historyLineMx)
	games := 0
	for n := 0; sc.Scan() && n < 2000; n++ {
		l := sc.Text()
		if _, ok := ParseEnterLeave(l, loc); ok || elRe.MatchString(l) {
			return "enterleave"
		}
		if _, ok := relativeSeconds(l); ok {
			if games++; games >= 3 {
				return "games"
			}
		}
	}
	if games > 0 {
		return "games"
	}
	return ""
}

const importBatch = 2000 // lines per transaction

// ImportEnterLeave imports the connections of an old enterleave.log: each join with its leave on the same slot.
func ImportEnterLeave(ctx context.Context, st *store.Store, serverID int64, key string, loc *time.Location, r io.Reader,
	p *ImportProgress, report func()) error {
	type pending struct {
		e  EnterLeave
		ip string
	}
	open := map[int]pending{}
	var batch []func(*store.Store) error
	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		fns := batch
		batch = nil
		var ferr error
		err := st.Ingest(serverID, key, p.Lines, "", func(b *store.Store) {
			for _, fn := range fns {
				if ferr == nil {
					ferr = fn(b)
				}
			}
		})
		if err == nil {
			err = ferr
		}
		report()
		return err
	}
	add := func(e EnterLeave, ip string, left int64, guid string) {
		batch = append(batch, func(b *store.Store) error {
			ok, err := b.AddPastConnection(serverID, e.Slot, e.Addr, ip, e.SteamID, guid, e.NameRaw, query.StripColors(e.NameRaw), e.TS.Unix(), left)
			if ok {
				p.Imported++
			} else if err == nil {
				p.Skipped++
			}
			return err
		})
	}
	err := readLines(r, func(l string, _ int64) error {
		p.Lines++
		e, ok := ParseEnterLeave(l, loc)
		if !ok {
			return nil
		}
		if e.Enter {
			if prev, ok := open[e.Slot]; ok { // joined again without a leave line: end unknown
				add(prev.e, prev.ip, 0, "")
			}
			open[e.Slot] = pending{e, query.HostOf(e.Addr)}
		} else if prev, ok := open[e.Slot]; ok {
			guid := e.PlayerID
			if guid == "0" {
				guid = ""
			}
			add(prev.e, prev.ip, e.TS.Unix(), guid)
			delete(open, e.Slot)
		}
		if len(batch) >= importBatch {
			if err := ctx.Err(); err != nil {
				return err
			}
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, prev := range open {
		add(prev.e, prev.ip, 0, "")
	}
	return flush()
}

// ImportGames imports an old games_mp.log: players, kills, chat and profiles, dated with the connections already
// stored (import the enterleave.log of the same time first). Runs of the server that can't be dated are counted
// in Undated.
func ImportGames(ctx context.Context, st *store.Store, serverID int64, key string, open func() (io.ReadCloser, error),
	p *ImportProgress, report func()) error {
	joins, leaves, err := st.Connections(serverID)
	if err != nil {
		return err
	}
	rd, err := open()
	if err != nil {
		return err
	}
	runs, err := anchors(rd, joins, leaves)
	rd.Close()
	if err != nil {
		return err
	}
	if rd, err = open(); err != nil {
		return err
	}
	defer rd.Close()

	var lines []string
	var stamps []int64
	flush := func() error {
		if len(lines) == 0 {
			return nil
		}
		ls, ts := lines, stamps
		lines, stamps = nil, nil
		var ferr error
		err := st.Ingest(serverID, key, p.Lines, "", func(b *store.Store) {
			for i, l := range ls {
				if ferr != nil {
					return
				}
				var stored bool
				stored, ferr = storePast(b, serverID, l, time.Unix(ts[i], 0), true)
				if stored {
					p.Imported++
				} else if ferr == nil && isEvent(l) {
					p.Skipped++
				}
			}
		})
		if err == nil {
			err = ferr
		}
		report()
		return err
	}
	run, last := -1, -1.0
	err = readLines(rd, func(l string, _ int64) error {
		p.Lines++
		rel, ok := relativeSeconds(l)
		if !ok {
			return nil
		}
		if last < 0 || rel < last {
			run++
		}
		last = rel
		if run >= len(runs) || runs[run] == noAnchor {
			if _, _, ok := ParseIdentity(l); ok || isEvent(l) {
				p.Undated++
			}
			return nil
		}
		lines, stamps = append(lines, l), append(stamps, runs[run]+int64(rel))
		if len(lines) >= importBatch {
			if err := ctx.Err(); err != nil {
				return err
			}
			return flush()
		}
		return nil
	})
	if err != nil {
		return err
	}
	return flush()
}

// isEvent: a kill, chat or profile line (what an import stores besides the names).
func isEvent(l string) bool {
	if _, ok := ParseChat(l); ok {
		return true
	}
	if _, ok := ParseKill(l); ok {
		return true
	}
	_, ok := ParseProfile(l)
	return ok
}

// ImportKey is the name under which the progress of an import job is checkpointed.
func ImportKey(id int64) string { return "import#" + strconv.FormatInt(id, 10) }
