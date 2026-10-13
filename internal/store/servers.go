package store

import (
	"database/sql"
	"errors"
)

// ServerConf is a monitored game server as set from the dashboard (or imported once from SERVERS).
// Passwords are decrypted for internal use; SQLite stores authenticated ciphertext.
type ServerConf struct {
	ID       int64
	Owner    string
	Name     string
	Host     string
	Port     int
	LogURL   string // ftp://user:pass@host:21/mods/<mod>/ or file:///<folder>/ ; "" = no logs
	RconPass string // "" = no rcon
	Enabled  bool
	Managed  bool
	Logs     LogFiles
}

// LogFiles are the advanced log settings of a server. Empty = default.
// A file is a name in the log folder, or an absolute path (on the FTP server, or on this machine for a local folder).
type LogFiles struct {
	Games      string // default games_mp.log
	Console    string // default qconsole.log
	EnterLeave string // default enterleave.log
	TZ         string // time zone of the server's clock (IANA name), default SERVER_LOG_TZ
	B3DB       string // SQLite database of the server's B3 admin bot (b3.db), "" = no B3
}

const serverCols = `id, name, host, port, COALESCE(log_url, ''), COALESCE(rcon_pass, ''), enabled, managed,
	COALESCE(games_log, ''), COALESCE(console_log, ''), COALESCE(enterleave_log, ''), COALESCE(log_tz, ''), COALESCE(b3_db, ''), owner`

func scanServer(sc interface{ Scan(...any) error }) (ServerConf, error) {
	var c ServerConf
	err := sc.Scan(&c.ID, &c.Name, &c.Host, &c.Port, &c.LogURL, &c.RconPass, &c.Enabled, &c.Managed,
		&c.Logs.Games, &c.Logs.Console, &c.Logs.EnterLeave, &c.Logs.TZ, &c.Logs.B3DB, &c.Owner)
	return c, err
}

// ServerConfs lists every server row, configured or only kept for its history.
func (s *Store) ServerConfs() ([]ServerConf, error) {
	rs, err := s.DB.Query(`SELECT ` + serverCols + ` FROM servers ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rs.Close()
	var out []ServerConf
	for rs.Next() {
		c, err := scanServer(rs)
		c, err = s.decodeServer(c, err)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rs.Err()
}

// ServerConf returns one server; ok=false if it doesn't exist.
func (s *Store) ServerConf(id int64) (ServerConf, bool, error) {
	c, err := scanServer(s.DB.QueryRow(`SELECT `+serverCols+` FROM servers WHERE id=?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return c, false, nil
	}
	c, err = s.decodeServer(c, err)
	return c, err == nil, err
}

// HasManagedServers: the servers have been set up (from the dashboard or by the one-time SERVERS import).
func (s *Store) HasManagedServers() (bool, error) {
	var n int
	err := s.DB.QueryRow(`SELECT COUNT(*) FROM servers WHERE managed=1`).Scan(&n)
	return n > 0, err
}

// ErrServerExists: another server already uses this address.
var ErrServerExists = errors.New("un autre serveur utilise déjà cette adresse")

// SaveServerConf creates (ID 0) or updates a server and marks it managed. A server row kept only for its history
// with the same address is taken over, so its history continues.
func (s *Store) SaveServerConf(c ServerConf) (int64, error) {
	var sealErr error
	c.LogURL, sealErr = s.seal(c.LogURL)
	if sealErr != nil {
		return 0, sealErr
	}
	c.RconPass, sealErr = s.seal(c.RconPass)
	if sealErr != nil {
		return 0, sealErr
	}
	c.Logs.B3DB, sealErr = s.seal(c.Logs.B3DB) // a MySQL address holds a password
	if sealErr != nil {
		return 0, sealErr
	}

	var other int64
	var managed bool
	var owner string
	err := s.DB.QueryRow(`SELECT id, managed, owner FROM servers WHERE host=? AND port=? AND id<>?`, c.Host, c.Port, c.ID).Scan(&other, &managed, &owner)
	switch {
	case err == nil && (managed || c.ID != 0 || owner != c.Owner):
		return 0, ErrServerExists
	case err == nil: // an old unmanaged row with this address: reuse it
		c.ID = other
	case !errors.Is(err, sql.ErrNoRows):
		return 0, err
	}
	if c.ID == 0 {
		err := s.DB.QueryRow(`INSERT INTO servers(name, host, port, log_url, rcon_pass, enabled, managed, games_log, console_log, enterleave_log, log_tz, b3_db, owner)
			VALUES(?,?,?,?,?,?,1,?,?,?,?,?,?) RETURNING id`,
			c.Name, c.Host, c.Port, c.LogURL, c.RconPass, c.Enabled, c.Logs.Games, c.Logs.Console, c.Logs.EnterLeave, c.Logs.TZ, c.Logs.B3DB, c.Owner).Scan(&c.ID)
		return c.ID, err
	}
	res, err := s.DB.Exec(`UPDATE servers SET name=?, host=?, port=?, log_url=?, rcon_pass=?, enabled=?, managed=1,
		games_log=?, console_log=?, enterleave_log=?, log_tz=?, b3_db=? WHERE id=?`,
		c.Name, c.Host, c.Port, c.LogURL, c.RconPass, c.Enabled, c.Logs.Games, c.Logs.Console, c.Logs.EnterLeave, c.Logs.TZ, c.Logs.B3DB, c.ID)
	if err != nil {
		return 0, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return 0, sql.ErrNoRows
	}
	return c.ID, nil
}

// SetServerEnabled starts (true) or stops (false) monitoring a server; its history stays.
func (s *Store) SetServerEnabled(id int64, enabled bool) error {
	_, err := s.DB.Exec(`UPDATE servers SET enabled=? WHERE id=? AND managed=1`, enabled, id)
	return err
}
