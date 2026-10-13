package store

import (
	"crypto/rand"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// !me in game gives a short code (and a link): typed on a phone, it opens the player's profile and pins it ("It's me").
// The code only points to the public profile, so it proves nothing and needs no secrecy beyond a day.

const (
	MeCodeTTL      = 24 * time.Hour
	meCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789" // no I, L, O, 0, 1: easy to read in the game font
	meCodeLen      = 5
)

// MeCode returns the code of a player, the same one while it is valid.
func (s *Store) MeCode(serverID int64, key, nameRaw string, now time.Time) (string, error) {
	if _, err := s.DB.Exec(`DELETE FROM me_codes WHERE created_at<?`, now.Add(-MeCodeTTL).Unix()); err != nil {
		return "", err
	}
	var code string
	err := s.DB.QueryRow(`SELECT code FROM me_codes WHERE server_id=? AND key=?`, serverID, key).Scan(&code)
	if err == nil {
		_, err = s.DB.Exec(`UPDATE me_codes SET name_raw=? WHERE code=?`, nameRaw, code)
		return code, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}
	for range 10 { // collisions are rare: 28 million codes for a few players a day
		b := make([]byte, meCodeLen)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		for i := range b {
			b[i] = meCodeAlphabet[int(b[i])%len(meCodeAlphabet)]
		}
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO me_codes VALUES(?,?,?,?,?)`, string(b), serverID, key, nameRaw, now.Unix())
		if err != nil {
			return "", err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return string(b), nil
		}
	}
	return "", errors.New("no free code")
}

// ClaimMe resolves a code (any case, spaces ignored); ok=false when unknown or expired.
func (s *Store) ClaimMe(code string, now time.Time) (serverID int64, key, nameRaw string, ok bool) {
	code = strings.ToUpper(strings.Join(strings.Fields(code), ""))
	if len(code) != meCodeLen {
		return 0, "", "", false
	}
	var name sql.NullString
	err := s.DB.QueryRow(`SELECT server_id, key, name_raw FROM me_codes WHERE code=? AND created_at>=?`, code, now.Add(-MeCodeTTL).Unix()).
		Scan(&serverID, &key, &name)
	return serverID, key, name.String, err == nil
}
