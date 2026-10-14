package store

import (
	"database/sql"
	"errors"
	"path"
	"regexp"
	"time"
)

// Server-side demos (CoD4X "record"): one .dm_1 file per player, seen from their eyes, in the server's demos/ folder.
const demosSchema = `
CREATE TABLE IF NOT EXISTS demo_settings(server_id INTEGER PRIMARY KEY, enabled INTEGER NOT NULL DEFAULT 0,
  min_players INTEGER NOT NULL DEFAULT 2, keep_hours INTEGER NOT NULL DEFAULT 48, dir TEXT NOT NULL DEFAULT '',
  reserve_gb REAL NOT NULL DEFAULT 0, updated_at INTEGER);
CREATE TABLE IF NOT EXISTS demos(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, file TEXT NOT NULL, player TEXT NOT NULL DEFAULT '',
  name_raw TEXT, started_at INTEGER NOT NULL, last_write INTEGER NOT NULL, size INTEGER NOT NULL DEFAULT 0,
  kept INTEGER NOT NULL DEFAULT 0, kept_reason TEXT, deleted_at INTEGER, UNIQUE(server_id, file));
CREATE INDEX IF NOT EXISTS demos_player ON demos(server_id, player, started_at);
`

// Demo keep levels: linked to a moderation item (kept DemoLinkedDays), or kept by hand (until unkept).
const (
	DemoLinked = 1
	DemoManual = 2
)

// DemoLinkedDays: how long a demo linked to a report, ban or alert is kept.
const DemoLinkedDays = 90

// DemoSettings of a server. Dir is the demos folder relative to the log folder (default DefaultDemoDir).
type DemoSettings struct {
	Enabled    bool    `json:"enabled"`
	MinPlayers int     `json:"min_players"`
	KeepHours  int     `json:"keep_hours"`
	Dir        string  `json:"dir"`
	ReserveGB  float64 `json:"reserve_gb"` // space set aside for the demos when the monitor can't see the disk (FTP); 0 = unknown
}

// DefaultDemoDir: CoD4X writes the demos in <fs_homepath>/demos, two levels above mods/<mod>/ where the logs are.
const DefaultDemoDir = "../../demos/"

func DefaultDemoSettings() DemoSettings { return DemoSettings{MinPlayers: 2, KeepHours: 48} }

func (s *Store) DemoSettings(id int64) (DemoSettings, error) {
	d := DefaultDemoSettings()
	err := s.DB.QueryRow(`SELECT enabled, min_players, keep_hours, dir, reserve_gb FROM demo_settings WHERE server_id=?`, id).
		Scan(&d.Enabled, &d.MinPlayers, &d.KeepHours, &d.Dir, &d.ReserveGB)
	if errors.Is(err, sql.ErrNoRows) {
		return d, nil
	}
	return d, err
}

func (s *Store) SaveDemoSettings(id int64, d DemoSettings) error {
	_, err := s.DB.Exec(`INSERT INTO demo_settings(server_id, enabled, min_players, keep_hours, dir, reserve_gb, updated_at) VALUES(?,?,?,?,?,?,?)
		ON CONFLICT(server_id) DO UPDATE SET enabled=excluded.enabled, min_players=excluded.min_players, keep_hours=excluded.keep_hours,
		dir=excluded.dir, reserve_gb=excluded.reserve_gb, updated_at=excluded.updated_at`,
		id, d.Enabled, d.MinPlayers, d.KeepHours, d.Dir, d.ReserveGB, time.Now().Unix())
	return err
}

// Demo is a demo file of the server.
type Demo struct {
	ID         int64  `json:"id"`
	File       string `json:"file"`
	Player     string `json:"player"` // GUID of the recorded player
	NameRaw    string `json:"name_raw"`
	StartedAt  int64  `json:"started_at"`
	LastWrite  int64  `json:"last_write"`
	Size       int64  `json:"size"`
	Kept       int    `json:"kept"`
	KeptReason string `json:"kept_reason,omitempty"`
	Deleted    bool   `json:"deleted"`
}

// demo_<guid>_0001.dm_1: the name the monitor gives (record <slot> demo_<guid>_) and the one of sv_autodemorecord.
var demoNameRe = regexp.MustCompile(`^demo_([0-9A-Za-z]+)_\d*\.dm_\d+$`)

// DemoPlayer reads the player's GUID from a demo file name ("" when it isn't one of ours).
func DemoPlayer(file string) string {
	if m := demoNameRe.FindStringSubmatch(path.Base(file)); m != nil {
		return m[1]
	}
	return ""
}

// SeeDemo records a demo file found in the folder (size and last write). A new one starts at its first sighting,
// or earlier when the console announced it (DemoStarted).
func (s *Store) SeeDemo(id int64, file string, size int64, mod, now time.Time) error {
	start := now.Unix()
	if !mod.IsZero() && mod.Before(now) && size > 0 {
		start = mod.Unix() // already written when first seen (the monitor was stopped): at least that old
	}
	player := DemoPlayer(file)
	_, err := s.DB.Exec(`INSERT INTO demos(server_id, file, player, name_raw, started_at, last_write, size) VALUES(?,?,?,
			(SELECT name_raw FROM player_names WHERE server_id=? AND guid=? ORDER BY last_seen DESC LIMIT 1),?,?,?)
		ON CONFLICT(server_id, file) DO UPDATE SET size=excluded.size, last_write=MAX(last_write, excluded.last_write), deleted_at=NULL`,
		id, file, player, id, player, start, max(mod.Unix(), start), size)
	return err
}

// DemoStarted: the console said "recording to demos/<file>" at ts.
func (s *Store) DemoStarted(id int64, file string, ts time.Time) error {
	player := DemoPlayer(file)
	_, err := s.DB.Exec(`INSERT INTO demos(server_id, file, player, name_raw, started_at, last_write) VALUES(?,?,?,
			(SELECT name_raw FROM player_names WHERE server_id=? AND guid=? ORDER BY last_seen DESC LIMIT 1),?,?)
		ON CONFLICT(server_id, file) DO UPDATE SET started_at=MIN(started_at, excluded.started_at), deleted_at=NULL`,
		id, file, player, id, player, ts.Unix(), ts.Unix())
	return err
}

// DemoGone: the file is no longer in the folder.
func (s *Store) DemoGone(id int64, file string, now time.Time) error {
	_, err := s.DB.Exec(`UPDATE demos SET deleted_at=? WHERE server_id=? AND file=? AND deleted_at IS NULL`, now.Unix(), id, file)
	return err
}

const demoCols = `id, file, player, COALESCE(name_raw, ''), started_at, last_write, size, kept, COALESCE(kept_reason, ''), deleted_at IS NOT NULL`

func scanDemos(rs *sql.Rows, err error) ([]Demo, error) {
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []Demo{}
	for rs.Next() {
		var d Demo
		if err := rs.Scan(&d.ID, &d.File, &d.Player, &d.NameRaw, &d.StartedAt, &d.LastWrite, &d.Size, &d.Kept, &d.KeptReason, &d.Deleted); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rs.Err()
}

// DemoFilter selects demos: one player, a time window (overlap), the deleted ones too.
type DemoFilter struct {
	Player         string
	From, To       int64
	Limit          int
	IncludeDeleted bool
}

func (s *Store) Demos(id int64, f DemoFilter) ([]Demo, error) {
	q, args := `SELECT `+demoCols+` FROM demos WHERE server_id=?`, []any{id}
	if !f.IncludeDeleted {
		q += ` AND deleted_at IS NULL`
	}
	if f.Player != "" {
		q, args = q+` AND player=?`, append(args, f.Player)
	}
	if f.From > 0 {
		q, args = q+` AND last_write>=?`, append(args, f.From)
	}
	if f.To > 0 {
		q, args = q+` AND started_at<=?`, append(args, f.To)
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 200
	}
	q, args = q+` ORDER BY started_at DESC, id DESC LIMIT ?`, append(args, f.Limit)
	return scanDemos(s.DB.Query(q, args...))
}

func (s *Store) Demo(id, demoID int64) (Demo, bool, error) {
	ds, err := scanDemos(s.DB.Query(`SELECT `+demoCols+` FROM demos WHERE server_id=? AND id=?`, id, demoID))
	if err != nil || len(ds) == 0 {
		return Demo{}, false, err
	}
	return ds[0], true, nil
}

// KeepDemo sets the keep level of a demo (0 = normal retention). A link never lowers a manual keep.
func (s *Store) KeepDemo(id, demoID int64, level int, reason string) error {
	_, err := s.DB.Exec(`UPDATE demos SET kept=?, kept_reason=? WHERE server_id=? AND id=? AND (?<>? OR kept<?)`,
		level, nullIf(reason), id, demoID, level, DemoLinked, DemoManual)
	return err
}

// LinkDemos keeps the demos of a player that cover the moment of a moderation item (from 30 min before to 10 min after).
func (s *Store) LinkDemos(id int64, player string, ts int64, reason string) ([]Demo, error) {
	if player == "" || ts <= 0 {
		return nil, nil
	}
	ds, err := s.Demos(id, DemoFilter{Player: player, From: ts - 30*60, To: ts + 10*60, Limit: 20, IncludeDeleted: true})
	if err != nil {
		return nil, err
	}
	for i, d := range ds {
		if d.Kept == 0 && !d.Deleted {
			if err := s.KeepDemo(id, d.ID, DemoLinked, reason); err != nil {
				return nil, err
			}
			ds[i].Kept, ds[i].KeptReason = DemoLinked, reason
		}
	}
	return ds, nil
}

// ExpiredDemos: files to delete now. Normal demos after keepHours without a write; linked ones after DemoLinkedDays.
func (s *Store) ExpiredDemos(id int64, keepHours int, now time.Time) ([]Demo, error) {
	return scanDemos(s.DB.Query(`SELECT `+demoCols+` FROM demos WHERE server_id=? AND deleted_at IS NULL AND
		((kept=0 AND last_write<?) OR (kept=? AND last_write<?)) ORDER BY last_write`,
		id, now.Add(-time.Duration(keepHours)*time.Hour).Unix(), DemoLinked, now.AddDate(0, 0, -DemoLinkedDays).Unix()))
}

// OldestDemos: the oldest normal demos, deleted first when the disk is nearly full.
func (s *Store) OldestDemos(id int64, n int) ([]Demo, error) {
	return scanDemos(s.DB.Query(`SELECT `+demoCols+` FROM demos WHERE server_id=? AND deleted_at IS NULL AND kept=0
		ORDER BY last_write LIMIT ?`, id, n))
}

// DemoUsage: what the demos of a server take now, and the measured size of a demo per hour of play
// (finished demos of at least 5 minutes, last 30 days; 0 when none yet).
type DemoUsage struct {
	Count      int     `json:"count"`
	Bytes      int64   `json:"bytes"`
	Kept       int     `json:"kept"`
	MBPerHour  float64 `json:"mb_per_hour"`
	Measured   int     `json:"measured"` // demos the rate comes from
	OldestTS   int64   `json:"oldest_ts"`
	RecordingN int     `json:"recording"` // written in the last 2 minutes
}

func (s *Store) DemoUsage(id int64, now time.Time) (DemoUsage, error) {
	var u DemoUsage
	err := s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(size), 0), COALESCE(SUM(kept>0), 0), COALESCE(MIN(started_at), 0),
		COALESCE(SUM(last_write>=?), 0) FROM demos WHERE server_id=? AND deleted_at IS NULL`, now.Add(-2*time.Minute).Unix(), id).
		Scan(&u.Count, &u.Bytes, &u.Kept, &u.OldestTS, &u.RecordingN)
	if err != nil {
		return u, err
	}
	var bytes, secs sql.NullFloat64
	err = s.DB.QueryRow(`SELECT SUM(size), SUM(last_write-started_at), COUNT(*) FROM demos WHERE server_id=? AND size>0
		AND last_write-started_at>=300 AND last_write<? AND started_at>=?`, id, now.Add(-2*time.Minute).Unix(), now.AddDate(0, 0, -30).Unix()).
		Scan(&bytes, &secs, &u.Measured)
	if err == nil && secs.Float64 > 0 {
		u.MBPerHour = bytes.Float64 / 1e6 / (secs.Float64 / 3600)
	}
	return u, err
}

// Attendance over the last 30 days, from the status polls: the share of time with at least min players on the
// server, and the average number of players during that time.
func (s *Store) DemoAttendance(id int64, min int, now time.Time) (hoursPerDay, avgPlayers float64, days float64, err error) {
	var total, above int
	var avg sql.NullFloat64
	var first sql.NullInt64
	err = s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(online=1 AND players>=?2), 0), AVG(CASE WHEN online=1 AND players>=?2 THEN players END), MIN(ts)
		FROM snapshots WHERE server_id=?1 AND ts>=?3`, id, min, now.AddDate(0, 0, -30).Unix()).Scan(&total, &above, &avg, &first)
	if err != nil || total == 0 {
		return 0, 0, 0, err
	}
	days = float64(now.Unix()-first.Int64) / 86400
	return 24 * float64(above) / float64(total), avg.Float64, days, nil
}
