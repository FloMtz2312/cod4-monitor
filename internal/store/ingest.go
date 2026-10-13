package store

import "database/sql"

type rowScanner interface{ Scan(...any) error }
type executor interface {
	Exec(string, ...any) (sql.Result, error)
	QueryRow(string, ...any) rowScanner
}
type trackedConn struct {
	db  *sql.DB
	tx  *sql.Tx
	err *error
}

func (c trackedConn) record(err error) {
	if err != nil && err != sql.ErrNoRows && c.err != nil && *c.err == nil {
		*c.err = err
	}
}
func (c trackedConn) Exec(q string, args ...any) (sql.Result, error) {
	var r sql.Result
	var err error
	if c.tx != nil {
		r, err = c.tx.Exec(q, args...)
	} else {
		r, err = c.db.Exec(q, args...)
	}
	c.record(err)
	return r, err
}

type checkedRow struct {
	row  *sql.Row
	conn trackedConn
}

func (r checkedRow) Scan(dest ...any) error {
	err := r.row.Scan(dest...)
	r.conn.record(err)
	return err
}
func (c trackedConn) QueryRow(q string, args ...any) rowScanner {
	if c.tx != nil {
		return checkedRow{c.tx.QueryRow(q, args...), c}
	}
	return checkedRow{c.db.QueryRow(q, args...), c}
}
func (s *Store) conn() executor { return trackedConn{db: s.DB, tx: s.tx, err: s.writeErr} }

// AfterCommit prevents chat commands, alerts and live events from escaping rolled-back batches.
func (s *Store) AfterCommit(fn func(*Store)) {
	if s.tx == nil {
		fn(s)
		return
	}
	*s.after = append(*s.after, fn)
}

// Ingest atomically writes all events and their checkpoint. Exec errors abort even if a legacy caller only logs them.
func (s *Store) Ingest(serverID int64, file string, off int64, state string, fn func(*Store)) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var writeErr error
	var after []func(*Store)
	batch := &Store{DB: s.DB, key: s.key, tx: tx, writeErr: &writeErr, after: &after}
	fn(batch)
	if writeErr != nil {
		return writeErr
	}
	if err = batch.SetLogOffset(serverID, file, off); err != nil {
		return err
	}
	if state != "" {
		if err = batch.SetCheckpoint(serverID, file, state); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	for _, f := range after {
		f(s)
	}
	return nil
}
func (s *Store) LogCheckpoint(serverID int64, file string) (string, error) {
	var state string
	err := s.DB.QueryRow(`SELECT state FROM log_checkpoint WHERE server_id=? AND file=?`, serverID, file).Scan(&state)
	if err == sql.ErrNoRows {
		err = nil
	}
	return state, err
}

func (s *Store) SetCheckpoint(serverID int64, file, state string) error {
	_, err := s.conn().Exec(`INSERT INTO log_checkpoint(server_id,file,state) VALUES(?,?,?) ON CONFLICT(server_id,file) DO UPDATE SET state=excluded.state`, serverID, file, state)
	return err
}
