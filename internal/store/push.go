package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"
)

// Push notifications: the phones (and browsers) of the staff that subscribed, with what each wants to receive.
// The permissions are checked when a notification is sent, not here.

type PushSub struct {
	Endpoint string   `json:"endpoint"` // URL of the push service, unique per browser
	Username string   `json:"-"`
	P256dh   string   `json:"-"` // browser's public key and secret (base64url), for the payload encryption
	Auth     string   `json:"-"`
	Lang     string   `json:"lang"`  // "fr", "en" or "de" (notifications in English)
	Types    []string `json:"types"` // down, report, suspect, b3ban
	Muted    []int64  `json:"muted"` // servers left out
	LastOK   int64    `json:"last_ok"`
}

const pushSubCols = `endpoint, username, p256dh, auth, lang, types, muted, COALESCE(last_ok, 0)`

func scanPushSub(sc interface{ Scan(...any) error }) (PushSub, error) {
	var p PushSub
	var types, muted string
	if err := sc.Scan(&p.Endpoint, &p.Username, &p.P256dh, &p.Auth, &p.Lang, &types, &muted, &p.LastOK); err != nil {
		return p, err
	}
	json.Unmarshal([]byte(types), &p.Types)
	json.Unmarshal([]byte(muted), &p.Muted)
	return p, nil
}

// SavePushSub adds or updates a browser (a phone signed in with another account now belongs to that account).
func (s *Store) SavePushSub(p PushSub, now time.Time) error {
	types, _ := json.Marshal(append([]string{}, p.Types...))
	muted, _ := json.Marshal(append([]int64{}, p.Muted...))
	_, err := s.DB.Exec(`INSERT INTO push_subs(endpoint, username, p256dh, auth, lang, types, muted, created_at) VALUES(?,?,?,?,?,?,?,?)
		ON CONFLICT(endpoint) DO UPDATE SET username=excluded.username, p256dh=excluded.p256dh, auth=excluded.auth, lang=excluded.lang,
		types=excluded.types, muted=excluded.muted, fails=0`, p.Endpoint, p.Username, p.P256dh, p.Auth, p.Lang, string(types), string(muted), now.Unix())
	return err
}

// PushSub: the browser of this endpoint; ok=false when it is not subscribed.
func (s *Store) PushSub(endpoint string) (PushSub, bool, error) {
	p, err := scanPushSub(s.DB.QueryRow(`SELECT `+pushSubCols+` FROM push_subs WHERE endpoint=?`, endpoint))
	if errors.Is(err, sql.ErrNoRows) {
		return p, false, nil
	}
	return p, err == nil, err
}

func (s *Store) PushSubs() ([]PushSub, error) {
	rs, err := s.DB.Query(`SELECT ` + pushSubCols + ` FROM push_subs`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []PushSub
	for rs.Next() {
		p, err := scanPushSub(rs)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rs.Err()
}

// DeletePushSub forgets a browser; with a username, only if it belongs to that account.
func (s *Store) DeletePushSub(endpoint, username string) error {
	q, args := `DELETE FROM push_subs WHERE endpoint=?`, []any{endpoint}
	if username != "" {
		q, args = q+` AND username=?`, append(args, username)
	}
	_, err := s.DB.Exec(q, args...)
	return err
}

// PushResult records a delivery: a success resets the failures, and a browser that failed 20 times in a row is
// forgotten (keys changed, app removed without the push service saying so).
func (s *Store) PushResult(endpoint string, ok bool, now time.Time) error {
	if ok {
		_, err := s.DB.Exec(`UPDATE push_subs SET last_ok=?, fails=0 WHERE endpoint=?`, now.Unix(), endpoint)
		return err
	}
	if _, err := s.DB.Exec(`UPDATE push_subs SET fails=fails+1 WHERE endpoint=?`, endpoint); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM push_subs WHERE endpoint=? AND fails>=20`, endpoint)
	return err
}

// PushSent marks an item of the moderation inbox as announced; false when it already was.
func (s *Store) PushSent(serverID int64, item string, now time.Time) (bool, error) {
	res, err := s.DB.Exec(`INSERT OR IGNORE INTO push_sent VALUES(?,?,?)`, serverID, item, now.Unix())
	if err != nil {
		return false, err
	}
	n, _ := res.RowsAffected()
	return n == 1, nil
}

// PurgePushSent forgets the announced items older than cutoff (the inbox only goes back 30 days).
func (s *Store) PurgePushSent(cutoff time.Time) error {
	_, err := s.DB.Exec(`DELETE FROM push_sent WHERE ts<?`, cutoff.Unix())
	return err
}
