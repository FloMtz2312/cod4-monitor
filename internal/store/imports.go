package store

import (
	"database/sql"
	"time"
)

// Old log files uploaded from the dashboard and imported in the background (games_mp.log or enterleave.log of a
// server, e.g. the file kept aside when the server started a new one).
const importsSchema = `
CREATE TABLE IF NOT EXISTS import_jobs(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, file_name TEXT NOT NULL,
  path TEXT NOT NULL, size INTEGER NOT NULL DEFAULT 0, kind TEXT NOT NULL DEFAULT '', state TEXT NOT NULL DEFAULT 'pending',
  lines INTEGER NOT NULL DEFAULT 0, imported INTEGER NOT NULL DEFAULT 0, skipped INTEGER NOT NULL DEFAULT 0,
  undated INTEGER NOT NULL DEFAULT 0, error TEXT NOT NULL DEFAULT '', username TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL, finished_at INTEGER);
CREATE INDEX IF NOT EXISTS import_jobs_srv ON import_jobs(server_id, id);
`

// Import job states.
const (
	ImportPending = "pending"
	ImportRunning = "running"
	ImportDone    = "done"
	ImportFailed  = "failed"
)

// ImportJob is one uploaded file. Lines read, lines imported, lines already known (skipped), player lines that
// couldn't be dated (games_mp.log only).
type ImportJob struct {
	ID         int64  `json:"id"`
	ServerID   int64  `json:"server_id"`
	FileName   string `json:"file_name"`
	Path       string `json:"-"`
	Size       int64  `json:"size"`
	Kind       string `json:"kind"` // "games", "enterleave", "" before it is read
	State      string `json:"state"`
	Lines      int64  `json:"lines"`
	Imported   int64  `json:"imported"`
	Skipped    int64  `json:"skipped"`
	Undated    int64  `json:"undated"`
	Error      string `json:"error,omitempty"`
	Username   string `json:"username"`
	CreatedAt  int64  `json:"created_at"`
	FinishedAt int64  `json:"finished_at,omitempty"`
}

const importCols = `id, server_id, file_name, path, size, kind, state, lines, imported, skipped, undated, error, username, created_at, COALESCE(finished_at, 0)`

func scanImport(sc interface{ Scan(...any) error }) (ImportJob, error) {
	var j ImportJob
	err := sc.Scan(&j.ID, &j.ServerID, &j.FileName, &j.Path, &j.Size, &j.Kind, &j.State, &j.Lines, &j.Imported, &j.Skipped,
		&j.Undated, &j.Error, &j.Username, &j.CreatedAt, &j.FinishedAt)
	return j, err
}

func (s *Store) AddImportJob(serverID int64, fileName, path string, size int64, user string, now time.Time) (int64, error) {
	r, err := s.DB.Exec(`INSERT INTO import_jobs(server_id, file_name, path, size, username, created_at) VALUES(?,?,?,?,?,?)`,
		serverID, fileName, path, size, user, now.Unix())
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// ImportJobs lists the last imports of a server, newest first.
func (s *Store) ImportJobs(serverID int64, limit int) ([]ImportJob, error) {
	rs, err := s.DB.Query(`SELECT `+importCols+` FROM import_jobs WHERE server_id=? ORDER BY id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []ImportJob{}
	for rs.Next() {
		j, err := scanImport(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rs.Err()
}

// NextImportJob is the oldest import waiting (found=false: none).
func (s *Store) NextImportJob() (ImportJob, bool, error) {
	j, err := scanImport(s.DB.QueryRow(`SELECT ` + importCols + ` FROM import_jobs WHERE state='pending' ORDER BY id LIMIT 1`))
	if err == sql.ErrNoRows {
		return j, false, nil
	}
	return j, err == nil, err
}

// SaveImportJob writes the progress and state of an import.
func (s *Store) SaveImportJob(j ImportJob) error {
	var finished any
	if j.FinishedAt > 0 {
		finished = j.FinishedAt
	}
	_, err := s.DB.Exec(`UPDATE import_jobs SET kind=?, state=?, lines=?, imported=?, skipped=?, undated=?, error=?, finished_at=? WHERE id=?`,
		j.Kind, j.State, j.Lines, j.Imported, j.Skipped, j.Undated, j.Error, finished, j.ID)
	return err
}

// RequeueImports puts the imports interrupted by a restart back in the queue (an import can run again: what is
// already stored is skipped).
func (s *Store) RequeueImports() error {
	_, err := s.DB.Exec(`UPDATE import_jobs SET state='pending' WHERE state='running'`)
	return err
}

// AddPastConnection stores a connection of an old enterleave.log, unless one of the same slot and name started
// within 5 s (already imported). left = 0: the end is unknown.
func (s *Store) AddPastConnection(serverID int64, slot int, addr, ip, steamID, guid, nameRaw, nameClean string, joined, left int64) (bool, error) {
	var one int
	err := s.conn().QueryRow(`SELECT 1 FROM connections WHERE server_id=? AND slot=? AND name_clean=? AND joined_at BETWEEN ? AND ? LIMIT 1`,
		serverID, slot, nameClean, joined-5, joined+5).Scan(&one)
	if err == nil {
		return false, nil
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	var leftAt any
	if left > 0 {
		leftAt = left
	}
	_, err = s.conn().Exec(`INSERT INTO connections(server_id, slot, addr, ip, steamid, guid, name_raw, name_clean, joined_at, left_at, open)
		VALUES(?,?,?,?,?,?,?,?,?,?,0)`, serverID, slot, addr, ip, nullIf(steamID), nullIf(guid), nameRaw, nameClean, joined, leftAt)
	return err == nil, err
}

// PastEventKnown tells whether a kill, chat message or profile of an imported line is already stored, within
// window seconds of ts (the dates of an import are estimated): kind is "kill", "chat" or "profile".
func (s *Store) PastEventKnown(serverID int64, kind string, ts, window int64, a, b, c string) (bool, error) {
	var q string
	switch kind {
	case "kill": // a = killer key ("" = none), b = victim key, c = weapon
		q = `SELECT 1 FROM kills WHERE server_id=?1 AND ts BETWEEN ?2 AND ?3 AND COALESCE(killer_key,'')=?4 AND victim_key=?5 AND COALESCE(weapon,'')=?6 LIMIT 1`
	case "chat": // a = clean name, b = message
		q = `SELECT 1 FROM chat_messages WHERE server_id=?1 AND ts BETWEEN ?2 AND ?3 AND name_clean=?4 AND message=?5 AND ?6=?6 LIMIT 1`
	case "profile": // a = player key
		q = `SELECT 1 FROM player_profiles WHERE server_id=?1 AND ts BETWEEN ?2 AND ?3 AND key=?4 AND ?5=?5 AND ?6=?6 LIMIT 1`
	default:
		return false, nil
	}
	var one int
	err := s.conn().QueryRow(q, serverID, ts-window, ts+window, a, b, c).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
