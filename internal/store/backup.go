package store

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
)

// VerifyBackup actually restores a copy in a temporary directory and checks its schema,
// SQLite integrity and ability to decrypt every configured server secret.
func (s *Store) VerifyBackup(path string) error {
	dir, err := os.MkdirTemp("", "cod4-restore-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	in, err := os.Open(path)
	if err != nil {
		return err
	}
	defer in.Close()
	restored := filepath.Join(dir, "restored.db")
	out, err := os.OpenFile(restored, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	ce := out.Close()
	if err != nil {
		return err
	}
	if ce != nil {
		return ce
	}
	u := url.URL{Scheme: "file", Path: restored, RawQuery: "mode=ro"}
	db, err := sql.Open("sqlite", u.String())
	if err != nil {
		return err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	var result string
	if err = db.QueryRow(`PRAGMA integrity_check`).Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("backup integrity: %s", result)
	}
	for _, table := range []string{"servers", "kills", "rounds", "connections", "log_pos", "sessions", "meta"} {
		var n int64
		if err = db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil {
			return err
		}
	}
	copy := &Store{DB: db, key: s.key}
	_, err = copy.ServerConfs()
	return err
}

func (s *Store) verifiedBackup(path string) error {
	if _, err := os.Stat(path); err == nil {
		return errors.New("backup already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	// A private directory prevents another local user reading VACUUM's temporary output.
	dir, err := os.MkdirTemp(filepath.Dir(path), ".backup-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	tmp := filepath.Join(dir, "copy.db")
	if _, err = s.DB.Exec(`VACUUM INTO ?`, tmp); err != nil {
		return err
	}
	if err = os.Chmod(tmp, 0600); err != nil {
		return err
	}
	if err = s.VerifyBackup(tmp); err != nil {
		return err
	}
	f, err := os.Open(tmp)
	if err != nil {
		return err
	}
	err = f.Sync()
	f.Close()
	if err != nil {
		return err
	}
	// Link is atomic and refuses to overwrite an existing backup.
	return os.Link(tmp, path)
}
