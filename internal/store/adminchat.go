package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Chat of a server's admins on the dashboard. An account takes part once linked to its player in game (a code shown on
// the dashboard, typed in game as !link <code>: the GUID comes from the server's log, so the link proves the player),
// and only while that player is a B3 admin (level 20 or more) of the server; the API checks the level.
const adminChatSchema = `
CREATE TABLE IF NOT EXISTS admin_links(username TEXT NOT NULL, server_id INTEGER NOT NULL, guid TEXT NOT NULL,
  name_raw TEXT NOT NULL DEFAULT '', linked_at INTEGER NOT NULL, PRIMARY KEY(username, server_id));
CREATE UNIQUE INDEX IF NOT EXISTS admin_links_guid ON admin_links(server_id, guid);
CREATE TABLE IF NOT EXISTS admin_link_codes(code TEXT PRIMARY KEY, username TEXT NOT NULL, server_id INTEGER NOT NULL,
  created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS admin_chat(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL,
  username TEXT NOT NULL, name_raw TEXT NOT NULL DEFAULT '', b3_id INTEGER NOT NULL DEFAULT 0, level INTEGER NOT NULL DEFAULT 0,
  body TEXT NOT NULL, mentions TEXT NOT NULL DEFAULT '[]');
CREATE INDEX IF NOT EXISTS admin_chat_srv ON admin_chat(server_id, id);
CREATE TABLE IF NOT EXISTS admin_chat_read(username TEXT NOT NULL, server_id INTEGER NOT NULL, last_id INTEGER NOT NULL,
  PRIMARY KEY(username, server_id));
`

const (
	AdminLinkTTL     = 10 * time.Minute
	adminLinkCodeLen = 6
)

// AdminLink: the player in game of a dashboard account on a server.
type AdminLink struct {
	Username string `json:"username"`
	ServerID int64  `json:"server_id"`
	GUID     string `json:"guid"`
	NameRaw  string `json:"name_raw"`
	LinkedAt int64  `json:"linked_at"`
}

// AdminLinkCode returns a new code for the account (the previous ones of this account and server stop working).
func (s *Store) AdminLinkCode(username string, serverID int64, now time.Time) (string, error) {
	if _, err := s.DB.Exec(`DELETE FROM admin_link_codes WHERE created_at<? OR (username=? AND server_id=?)`,
		now.Add(-AdminLinkTTL).Unix(), username, serverID); err != nil {
		return "", err
	}
	for range 10 {
		b := make([]byte, adminLinkCodeLen)
		if _, err := rand.Read(b); err != nil {
			return "", err
		}
		for i := range b {
			b[i] = meCodeAlphabet[int(b[i])%len(meCodeAlphabet)]
		}
		res, err := s.DB.Exec(`INSERT OR IGNORE INTO admin_link_codes VALUES(?,?,?,?)`, string(b), username, serverID, now.Unix())
		if err != nil {
			return "", err
		}
		if n, _ := res.RowsAffected(); n == 1 {
			return string(b), nil
		}
	}
	return "", errors.New("no free code")
}

// UseAdminLinkCode links the player who typed the code in game on this server to the account that asked for it. The code
// is used once; the GUID leaves any account it was linked to. ok=false: unknown, expired or another server's code.
func (s *Store) UseAdminLinkCode(serverID int64, code, guid, nameRaw string, now time.Time) (username string, ok bool, err error) {
	code = strings.ToUpper(strings.Join(strings.Fields(code), ""))
	if len(code) != adminLinkCodeLen || guid == "" || guid == "0" {
		return "", false, nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return "", false, err
	}
	defer tx.Rollback()
	err = tx.QueryRow(`SELECT username FROM admin_link_codes WHERE code=? AND server_id=? AND created_at>=?`,
		code, serverID, now.Add(-AdminLinkTTL).Unix()).Scan(&username)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`DELETE FROM admin_link_codes WHERE code=?`, []any{code}},
		{`DELETE FROM admin_links WHERE server_id=? AND (guid=? OR username=?)`, []any{serverID, guid, username}},
		{`INSERT INTO admin_links VALUES(?,?,?,?,?)`, []any{username, serverID, guid, nameRaw, now.Unix()}},
		// a new admin starts with the history read
		{`INSERT OR IGNORE INTO admin_chat_read SELECT ?, ?, COALESCE(MAX(id), 0) FROM admin_chat WHERE server_id=?`, []any{username, serverID, serverID}},
	} {
		if _, err := tx.Exec(q.sql, q.args...); err != nil {
			return "", false, err
		}
	}
	return username, true, tx.Commit()
}

// AdminLinkOf: the player linked to the account on the server.
func (s *Store) AdminLinkOf(username string, serverID int64) (AdminLink, bool, error) {
	l := AdminLink{Username: username, ServerID: serverID}
	err := s.DB.QueryRow(`SELECT guid, name_raw, linked_at FROM admin_links WHERE username=? AND server_id=?`, username, serverID).
		Scan(&l.GUID, &l.NameRaw, &l.LinkedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return l, false, nil
	}
	return l, err == nil, err
}

// AdminLinks: the linked accounts of the server (the API keeps those that still have access to it).
func (s *Store) AdminLinks(serverID int64) ([]AdminLink, error) {
	rs, err := s.DB.Query(`SELECT username, guid, name_raw, linked_at FROM admin_links WHERE server_id=? ORDER BY username`, serverID)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []AdminLink
	for rs.Next() {
		l := AdminLink{ServerID: serverID}
		if err := rs.Scan(&l.Username, &l.GUID, &l.NameRaw, &l.LinkedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rs.Err()
}

func (s *Store) DeleteAdminLink(username string, serverID int64) error {
	_, err := s.DB.Exec(`DELETE FROM admin_links WHERE username=? AND server_id=?`, username, serverID)
	return err
}

// AdminMessage: one message of the admins' chat. Mentions: the accounts it notified.
type AdminMessage struct {
	ID       int64    `json:"id"`
	TS       int64    `json:"ts"`
	Username string   `json:"username"`
	NameRaw  string   `json:"name_raw"`
	B3ID     int64    `json:"b3_id"`
	Level    int      `json:"level"`
	Body     string   `json:"body"`
	Mentions []string `json:"mentions"`
}

func (s *Store) AddAdminMessage(serverID int64, m AdminMessage) (int64, error) {
	mentions, _ := json.Marshal(append([]string{}, m.Mentions...))
	r, err := s.DB.Exec(`INSERT INTO admin_chat(server_id, ts, username, name_raw, b3_id, level, body, mentions) VALUES(?,?,?,?,?,?,?,?)`,
		serverID, m.TS, m.Username, m.NameRaw, m.B3ID, m.Level, m.Body, string(mentions))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// AdminMessages: the messages after `after`, or the last `limit` ones before `before` (0 = the latest), oldest first.
func (s *Store) AdminMessages(serverID, after, before int64, limit int) ([]AdminMessage, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	q := `SELECT id, ts, username, name_raw, b3_id, level, body, mentions FROM admin_chat WHERE server_id=? AND id>? ORDER BY id LIMIT ?`
	args := []any{serverID, after, limit}
	if after <= 0 {
		if before <= 0 {
			before = 1 << 62
		}
		q = `SELECT * FROM (SELECT id, ts, username, name_raw, b3_id, level, body, mentions FROM admin_chat WHERE server_id=? AND id<?
			ORDER BY id DESC LIMIT ?) ORDER BY id`
		args = []any{serverID, before, limit}
	}
	rs, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	out := []AdminMessage{}
	for rs.Next() {
		var m AdminMessage
		var mentions string
		if err := rs.Scan(&m.ID, &m.TS, &m.Username, &m.NameRaw, &m.B3ID, &m.Level, &m.Body, &mentions); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(mentions), &m.Mentions)
		out = append(out, m)
	}
	return out, rs.Err()
}

// AdminChatRead marks the messages up to id as read by the account (never backwards).
func (s *Store) AdminChatRead(username string, serverID, id int64) error {
	_, err := s.DB.Exec(`INSERT INTO admin_chat_read VALUES(?,?,?) ON CONFLICT(username, server_id) DO UPDATE SET last_id=MAX(last_id, excluded.last_id)`,
		username, serverID, id)
	return err
}

// AdminChatUnread: the messages of the others not read yet by the account, and among them those that mention it.
func (s *Store) AdminChatUnread(username string, serverID int64) (unread, mentions int64, err error) {
	var last int64
	if err := s.DB.QueryRow(`SELECT last_id FROM admin_chat_read WHERE username=? AND server_id=?`, username, serverID).Scan(&last); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, 0, err
	}
	err = s.DB.QueryRow(`SELECT COUNT(*), COALESCE(SUM(EXISTS(SELECT 1 FROM json_each(mentions) WHERE value=?2)), 0)
		FROM admin_chat WHERE server_id=?1 AND id>?3 AND username<>?2`, serverID, username, last).Scan(&unread, &mentions)
	return unread, mentions, err
}
