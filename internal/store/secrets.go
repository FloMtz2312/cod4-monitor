package store

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"strings"
)

const sealedPrefix = "enc:v1:"

// The key is separate from SQLite and its backups. Existing installs migrate automatically.
func secretKey(path string) ([]byte, error) {
	if key := os.Getenv("MONITOR_SECRET_KEY"); key != "" {
		b, err := base64.StdEncoding.DecodeString(key)
		if err != nil || len(b) != 32 {
			return nil, errors.New("MONITOR_SECRET_KEY must be base64 of 32 random bytes")
		}
		return b, nil
	}
	if path == ":memory:" {
		b := make([]byte, 32)
		rand.Read(b)
		return b, nil
	}
	name := path + ".key"
	b, err := os.ReadFile(name)
	if errors.Is(err, os.ErrNotExist) {
		b = make([]byte, 32)
		rand.Read(b)
		f, e := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if e != nil {
			return nil, e
		}
		_, e = f.Write(b)
		if e == nil {
			e = f.Sync()
		}
		ce := f.Close()
		if e != nil {
			return nil, e
		}
		if ce != nil {
			return nil, ce
		}
	} else if err != nil {
		return nil, err
	}
	if len(b) != 32 {
		return nil, errors.New("invalid database encryption key")
	}
	if err := os.Chmod(name, 0600); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) seal(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	n := make([]byte, a.NonceSize())
	rand.Read(n)
	return sealedPrefix + base64.StdEncoding.EncodeToString(a.Seal(n, n, []byte(value), []byte(sealedPrefix))), nil
}
func (s *Store) unseal(value string) (string, error) {
	if !strings.HasPrefix(value, sealedPrefix) {
		return value, nil
	}
	b, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(value, sealedPrefix))
	if err != nil {
		return "", errors.New("invalid encrypted secret")
	}
	block, err := aes.NewCipher(s.key)
	if err != nil {
		return "", err
	}
	a, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(b) < a.NonceSize() {
		return "", errors.New("invalid encrypted secret")
	}
	plain, err := a.Open(nil, b[:a.NonceSize()], b[a.NonceSize():], []byte(sealedPrefix))
	if err != nil {
		return "", errors.New("cannot decrypt server secrets; restore the matching .key file")
	}
	return string(plain), nil
}
func (s *Store) decodeServer(c ServerConf, err error) (ServerConf, error) {
	if err != nil {
		return c, err
	}
	if c.LogURL, err = s.unseal(c.LogURL); err != nil {
		return c, err
	}
	if c.RconPass, err = s.unseal(c.RconPass); err != nil {
		return c, err
	}
	c.Logs.B3DB, err = s.unseal(c.Logs.B3DB)
	return c, err
}
func (s *Store) migrateSecrets() error {
	rows, err := s.DB.Query(`SELECT id,COALESCE(log_url,''),COALESCE(rcon_pass,'') FROM servers`)
	if err != nil {
		return err
	}
	type update struct {
		id        int64
		log, pass string
	}
	var changes []update
	for rows.Next() {
		var u update
		if err = rows.Scan(&u.id, &u.log, &u.pass); err != nil {
			rows.Close()
			return err
		}
		changed := false
		for _, value := range []*string{&u.log, &u.pass} {
			if strings.HasPrefix(*value, sealedPrefix) {
				if _, err = s.unseal(*value); err != nil {
					rows.Close()
					return err
				}
			} else if *value != "" {
				*value, err = s.seal(*value)
				if err != nil {
					rows.Close()
					return err
				}
				changed = true
			}
		}
		if changed {
			changes = append(changes, u)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if len(changes) == 0 {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, u := range changes {
		if _, err = tx.Exec(`UPDATE servers SET log_url=?,rcon_pass=? WHERE id=?`, u.log, u.pass, u.id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	// secure_delete removes old cells; checkpoint removes the pre-migration WAL pages.
	_, err = s.DB.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}
