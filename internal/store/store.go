// Package store persists poll results in SQLite. All timestamps are unix seconds.
package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"

	"cod4monitor/internal/query"

	_ "modernc.org/sqlite"
)

const schema = `
CREATE TABLE IF NOT EXISTS servers(id INTEGER PRIMARY KEY, name TEXT NOT NULL, host TEXT NOT NULL, port INTEGER NOT NULL, UNIQUE(host, port));
CREATE TABLE IF NOT EXISTS snapshots(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, online INTEGER NOT NULL,
  latency_ms REAL, hostname TEXT, map TEXT, gametype TEXT, players INTEGER, max_players INTEGER);
CREATE INDEX IF NOT EXISTS snapshots_srv_ts ON snapshots(server_id, ts);
CREATE TABLE IF NOT EXISTS player_sightings(snapshot_id INTEGER NOT NULL, name_raw TEXT, name_clean TEXT, score INTEGER, ping INTEGER);
CREATE INDEX IF NOT EXISTS sightings_snap ON player_sightings(snapshot_id);
CREATE TABLE IF NOT EXISTS outages(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, started_at INTEGER NOT NULL, ended_at INTEGER);
CREATE TABLE IF NOT EXISTS player_sessions(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, name_raw TEXT NOT NULL, name_clean TEXT NOT NULL,
  started_at INTEGER NOT NULL, last_seen INTEGER NOT NULL, best_score INTEGER NOT NULL DEFAULT 0, open INTEGER NOT NULL DEFAULT 1);
CREATE INDEX IF NOT EXISTS sessions_srv_name ON player_sessions(server_id, name_clean);
CREATE TABLE IF NOT EXISTS cvar_history(server_id INTEGER NOT NULL, ts INTEGER NOT NULL, vars TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS chat_messages(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, guid TEXT,
  name_raw TEXT NOT NULL, name_clean TEXT NOT NULL, team INTEGER NOT NULL, message TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS chat_srv_name ON chat_messages(server_id, name_clean, ts);
CREATE INDEX IF NOT EXISTS chat_srv_ts ON chat_messages(server_id, ts);
CREATE TABLE IF NOT EXISTS player_names(server_id INTEGER NOT NULL, guid TEXT NOT NULL, name_clean TEXT NOT NULL, name_raw TEXT NOT NULL,
  first_seen INTEGER NOT NULL, last_seen INTEGER NOT NULL, PRIMARY KEY(server_id, guid, name_clean));
CREATE INDEX IF NOT EXISTS names_srv_name ON player_names(server_id, name_clean);
CREATE TABLE IF NOT EXISTS kills(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, round_id INTEGER,
  killer_key TEXT, killer_guid TEXT, killer_name TEXT, killer_team TEXT,
  victim_key TEXT NOT NULL, victim_guid TEXT, victim_name TEXT NOT NULL, victim_team TEXT,
  weapon TEXT, cause TEXT, hitloc TEXT, damage INTEGER, teamkill INTEGER NOT NULL DEFAULT 0);
CREATE INDEX IF NOT EXISTS kills_srv_ts ON kills(server_id, ts);
CREATE INDEX IF NOT EXISTS kills_killer ON kills(server_id, killer_key);
CREATE INDEX IF NOT EXISTS kills_victim ON kills(server_id, victim_key);
CREATE TABLE IF NOT EXISTS rounds(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, started_at INTEGER NOT NULL, ended_at INTEGER, map TEXT, gametype TEXT);
CREATE INDEX IF NOT EXISTS rounds_srv ON rounds(server_id, started_at);
CREATE TABLE IF NOT EXISTS rcon_log(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, command TEXT NOT NULL, output TEXT, error TEXT);
CREATE TABLE IF NOT EXISTS bans(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, guid TEXT, name_raw TEXT, ip TEXT,
  duration TEXT, reason TEXT, command TEXT);
CREATE TABLE IF NOT EXISTS console_errors(server_id INTEGER NOT NULL, message TEXT NOT NULL, count INTEGER NOT NULL,
  first_seen INTEGER NOT NULL, last_seen INTEGER NOT NULL, PRIMARY KEY(server_id, message));
CREATE TABLE IF NOT EXISTS accuracy(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, round_id INTEGER,
  key TEXT NOT NULL, guid TEXT, name_raw TEXT, shots INTEGER, hits INTEGER, dmg_done INTEGER, dmg_taken INTEGER, ff_done INTEGER, ff_taken INTEGER);
CREATE INDEX IF NOT EXISTS accuracy_srv_key ON accuracy(server_id, key, ts);
CREATE TABLE IF NOT EXISTS player_notes(server_id INTEGER NOT NULL, key TEXT NOT NULL, tag TEXT, note TEXT, updated_at INTEGER, PRIMARY KEY(server_id, key));
CREATE TABLE IF NOT EXISTS schedules(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, command TEXT NOT NULL,
  every_min INTEGER, at_time TEXT, enabled INTEGER NOT NULL DEFAULT 1, last_run INTEGER);
CREATE TABLE IF NOT EXISTS admin_log(id INTEGER PRIMARY KEY, ts INTEGER NOT NULL, server_id INTEGER, username TEXT NOT NULL,
  action TEXT NOT NULL, detail TEXT);
CREATE TABLE IF NOT EXISTS users(username TEXT PRIMARY KEY, pass_hash TEXT NOT NULL, created_at INTEGER, last_login INTEGER);
CREATE TABLE IF NOT EXISTS server_members(server_id INTEGER NOT NULL, username TEXT NOT NULL, permissions TEXT NOT NULL DEFAULT '[]', PRIMARY KEY(server_id,username));
CREATE TABLE IF NOT EXISTS invitations(token_hash TEXT PRIMARY KEY, username TEXT NOT NULL, account_role TEXT NOT NULL, server_id INTEGER, permissions TEXT NOT NULL DEFAULT '[]', created_by TEXT NOT NULL, created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, used_at INTEGER, revoked_at INTEGER);
CREATE INDEX IF NOT EXISTS invitations_creator ON invitations(created_by,created_at);
CREATE TABLE IF NOT EXISTS sessions(token_hash TEXT PRIMARY KEY, username TEXT NOT NULL, created_at INTEGER, expires_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS log_checkpoint(server_id INTEGER NOT NULL, file TEXT NOT NULL, state TEXT NOT NULL, PRIMARY KEY(server_id,file));
CREATE TABLE IF NOT EXISTS log_pos(server_id INTEGER NOT NULL, file TEXT NOT NULL, pos INTEGER NOT NULL, PRIMARY KEY(server_id, file));
CREATE TABLE IF NOT EXISTS connections(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, slot INTEGER NOT NULL, addr TEXT NOT NULL, ip TEXT NOT NULL,
  steamid TEXT, guid TEXT, name_raw TEXT NOT NULL, name_clean TEXT NOT NULL, joined_at INTEGER NOT NULL, left_at INTEGER, open INTEGER NOT NULL DEFAULT 1);
CREATE INDEX IF NOT EXISTS conn_srv_guid ON connections(server_id, guid);
CREATE INDEX IF NOT EXISTS conn_srv_open ON connections(server_id, open, slot);
CREATE TABLE IF NOT EXISTS game_events(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, round_id INTEGER,
  type TEXT NOT NULL, key TEXT, name_raw TEXT, value INTEGER);
CREATE INDEX IF NOT EXISTS events_srv ON game_events(server_id, type, ts);
CREATE TABLE IF NOT EXISTS match_scores(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, round_id INTEGER,
  key TEXT NOT NULL, name_raw TEXT, team TEXT, score INTEGER, kills INTEGER, deaths INTEGER, level INTEGER, prestige INTEGER);
CREATE INDEX IF NOT EXISTS mscores_srv ON match_scores(server_id, key, ts);
CREATE TABLE IF NOT EXISTS match_results(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, round_id INTEGER,
  winner TEXT, score_allies INTEGER, score_axis INTEGER);
CREATE TABLE IF NOT EXISTS map_votes(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, map TEXT NOT NULL, mode TEXT,
  votes INTEGER, chosen INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS reports(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, reporter_key TEXT, reporter_name TEXT,
  target TEXT, reason TEXT, handled_by TEXT, handled_at INTEGER);
CREATE TABLE IF NOT EXISTS player_profiles(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, key TEXT NOT NULL,
  name_raw TEXT, playerid TEXT, level INTEGER, prestige INTEGER, kills INTEGER, deaths INTEGER, headshots INTEGER, matches INTEGER);
CREATE INDEX IF NOT EXISTS profiles_srv_key ON player_profiles(server_id, key, ts);
CREATE TABLE IF NOT EXISTS player_fingerprints(id INTEGER PRIMARY KEY, server_id INTEGER NOT NULL, ts INTEGER NOT NULL, key TEXT NOT NULL,
  rate TEXT, snaps TEXT, maxpackets TEXT, userinfo TEXT);
CREATE INDEX IF NOT EXISTS fingerprints_srv_key ON player_fingerprints(server_id, key, ts);
CREATE TABLE IF NOT EXISTS welcome_messages(server_id INTEGER NOT NULL, key TEXT NOT NULL, message TEXT NOT NULL, updated_at INTEGER,
  PRIMARY KEY(server_id, key));
CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS inbox_dismissed(server_id INTEGER NOT NULL, item TEXT NOT NULL, username TEXT, ts INTEGER NOT NULL,
  PRIMARY KEY(server_id, item));
CREATE TABLE IF NOT EXISTS me_codes(code TEXT PRIMARY KEY, server_id INTEGER NOT NULL, key TEXT NOT NULL, name_raw TEXT,
  created_at INTEGER NOT NULL);
CREATE TABLE IF NOT EXISTS push_subs(endpoint TEXT PRIMARY KEY, username TEXT NOT NULL, p256dh TEXT NOT NULL, auth TEXT NOT NULL,
  lang TEXT NOT NULL DEFAULT 'fr', types TEXT NOT NULL DEFAULT '[]', muted TEXT NOT NULL DEFAULT '[]', created_at INTEGER NOT NULL,
  last_ok INTEGER, fails INTEGER NOT NULL DEFAULT 0);
CREATE TABLE IF NOT EXISTS push_sent(server_id INTEGER NOT NULL, item TEXT NOT NULL, ts INTEGER NOT NULL, PRIMARY KEY(server_id, item));
`

// columns added after the first release (CREATE TABLE IF NOT EXISTS doesn't add them to an existing table)
var columns = []struct{ table, column, def string }{
	{"kills", "distance", "REAL"},  // meters, from the mod's KE line
	{"kills", "tags", "TEXT"},      // hs,wb,ns,qs,ls,air,last,collat,triple
	{"kills", "points", "INTEGER"}, // KillEvent points
	{"kills", "ads_ms", "INTEGER"}, // time aiming before the shot, -1 = not aiming
	{"rounds", "winner", "TEXT"},   // allies / axis / tie / ffa, from the RE line
	{"rounds", "score_allies", "INTEGER"},
	{"rounds", "score_axis", "INTEGER"},
	{"rounds", "attackers", "TEXT"},
	{"outages", "reason", "TEXT"},     // last error line of the console before the server went down
	{"outages", "console", "TEXT"},    // last console lines before it
	{"outages", "restart", "INTEGER"}, // 1: the server restarted between two polls (never seen offline)
	{"outages", "clean", "INTEGER"},   // 1: normal shutdown (quit, host restart), not a crash
	// servers configured from the dashboard (the SERVERS variable is only imported once)
	{"servers", "log_url", "TEXT"},                       // ftp://user:pass@host:21/mods/<mod>/ or file:///<folder>/, empty = no logs
	{"servers", "rcon_pass", "TEXT"},                     // empty = no rcon
	{"servers", "enabled", "INTEGER NOT NULL DEFAULT 1"}, // 0: not monitored any more, history kept
	{"servers", "managed", "INTEGER NOT NULL DEFAULT 0"}, // 1: monitored with the settings above
	// advanced settings, empty = default: log file names or paths, time zone of the server's clock
	{"servers", "games_log", "TEXT"},
	{"servers", "console_log", "TEXT"},
	{"servers", "enterleave_log", "TEXT"},
	{"servers", "log_tz", "TEXT"},
	{"servers", "owner", "TEXT NOT NULL DEFAULT ''"},
	{"users", "account_role", "TEXT NOT NULL DEFAULT 'moderator'"},
	{"servers", "b3_db", "TEXT"},      // SQLite database of the server's B3 admin bot, inside B3_ROOT; empty = no B3
	{"servers", "accent", "TEXT"},     // color of the server in the interface (#RRGGBB), empty = default
	{"servers", "logo", "BLOB"},       // PNG, square, empty = initials
	{"servers", "logo_ts", "INTEGER"}, // when the logo changed: version of its URL
}

type Store struct {
	DB       *sql.DB
	key      []byte
	tx       *sql.Tx
	writeErr *error
	after    *[]func(*Store)
}

// Meta reads a small piece of state kept across restarts (last weekly mail, monitor starts...). "" if unset.
func (s *Store) Meta(key string) string {
	var v string
	s.conn().QueryRow(`SELECT value FROM meta WHERE key=?`, key).Scan(&v)
	return v
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.conn().Exec(`INSERT INTO meta(key, value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func Open(path string) (*Store, error) {
	if path != ":memory:" {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		f.Close()
		if err = os.Chmod(path, 0600); err != nil {
			return nil, err
		}
	}

	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=secure_delete(ON)")
	if err != nil {
		return nil, err
	}
	success := false
	defer func() {
		if !success {
			db.Close()
		}
	}()
	db.SetMaxOpenConns(1) // ponytail: one connection serializes everything; fine for a handful of servers.
	if _, err := db.Exec(schema + telemetrySchema + performanceSchema + demosSchema + importsSchema); err != nil {
		return nil, err
	}
	for _, c := range columns {
		var n int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name=?`, c.table, c.column).Scan(&n); err != nil {
			return nil, err
		}
		if n == 0 {
			if _, err := db.Exec(`ALTER TABLE ` + c.table + ` ADD COLUMN ` + c.column + ` ` + c.def); err != nil {
				return nil, err
			}
		}
	}
	// v1 kept a single log position per server (games_mp.log).
	if _, err := db.Exec(`SELECT 1 FROM log_state LIMIT 1`); err == nil {
		if _, err := db.Exec(`INSERT OR IGNORE INTO log_pos SELECT server_id, 'games_mp.log', pos FROM log_state; DROP TABLE log_state;`); err != nil {
			return nil, err
		}
	}
	// Sessions left open by a previous run can't be trusted anymore.
	if _, err := db.Exec(`UPDATE player_sessions SET open=0 WHERE open=1`); err != nil {
		return nil, err
	}
	key, err := secretKey(path)
	if err != nil {
		db.Close()
		return nil, err
	}
	st := &Store{DB: db, key: key}
	if err = st.migrateSecrets(); err != nil {
		db.Close()
		return nil, err
	}
	if err = st.prepareAggregates(); err != nil {
		db.Close()
		return nil, err
	}
	st.Optimize()
	success = true
	return st, nil
}

// UpsertServer returns the id of host:port, creating or renaming it.
func (s *Store) UpsertServer(name, host string, port int) (int64, error) {
	var id int64
	err := s.conn().QueryRow(`INSERT INTO servers(name, host, port) VALUES(?,?,?)
		ON CONFLICT(host, port) DO UPDATE SET name=excluded.name RETURNING id`, name, host, port).Scan(&id)
	return id, err
}

// Record stores one poll result (st == nil means offline) and updates outages and player sessions.
func (s *Store) Record(serverID int64, ts time.Time, st *query.Status) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := ts.Unix()

	if st == nil {
		if _, err := tx.Exec(`INSERT INTO snapshots(server_id, ts, online) VALUES(?,?,0)`, serverID, now); err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO outages(server_id, started_at)
			SELECT ?, ? WHERE NOT EXISTS (SELECT 1 FROM outages WHERE server_id=? AND ended_at IS NULL)`, serverID, now, serverID); err != nil {
			return err
		}
		if _, err := tx.Exec(`UPDATE player_sessions SET open=0 WHERE server_id=? AND open=1`, serverID); err != nil {
			return err
		}
		// Server down: nobody will "leave" in enterleave.log. They left when the server crashed,
		// i.e. at the last successful poll. Earlier than joined_at means unknown (NULL, not counted).
		if _, err := tx.Exec(`UPDATE connections SET open=0,
			left_at=(SELECT MAX(ts) FROM snapshots WHERE server_id=?1 AND online=1 AND ts>=joined_at)
			WHERE server_id=?1 AND open=1`, serverID); err != nil {
			return err
		}
		return tx.Commit()
	}

	res, err := tx.Exec(`INSERT INTO snapshots(server_id, ts, online, latency_ms, hostname, map, gametype, players, max_players)
		VALUES(?,?,1,?,?,?,?,?,?)`, serverID, now, float64(st.Latency.Microseconds())/1000, st.Hostname, st.Map, st.Gametype, len(st.Players), st.MaxPlayers)
	if err != nil {
		return err
	}
	snapID, _ := res.LastInsertId()
	if _, err := tx.Exec(`UPDATE outages SET ended_at=? WHERE server_id=? AND ended_at IS NULL`, now, serverID); err != nil {
		return err
	}

	seen := map[string]bool{}
	for _, p := range st.Players {
		if _, err := tx.Exec(`INSERT INTO player_sightings VALUES(?,?,?,?,?)`, snapID, p.NameRaw, p.NameClean, p.Score, p.Ping); err != nil {
			return err
		}
		if seen[p.NameRaw] {
			continue
		}
		seen[p.NameRaw] = true
		r, err := tx.Exec(`UPDATE player_sessions SET last_seen=?, best_score=MAX(best_score, ?) WHERE server_id=? AND name_raw=? AND open=1`,
			now, p.Score, serverID, p.NameRaw)
		if err != nil {
			return err
		}
		if n, _ := r.RowsAffected(); n == 0 {
			if _, err := tx.Exec(`INSERT INTO player_sessions(server_id, name_raw, name_clean, started_at, last_seen, best_score) VALUES(?,?,?,?,?,?)`,
				serverID, p.NameRaw, p.NameClean, now, now, p.Score); err != nil {
				return err
			}
		}
	}
	// Close sessions of players who left.
	if _, err := tx.Exec(`UPDATE player_sessions SET open=0 WHERE server_id=? AND open=1 AND last_seen<?`, serverID, now); err != nil {
		return err
	}

	// Keep full cvar dump only when it changed (ignoring the ever-ticking "uptime").
	vars := map[string]string{}
	for k, v := range st.Vars {
		if k != "uptime" && !query.SecretField(k) {
			vars[k] = v
		}
	}
	js, _ := json.Marshal(vars) // map keys are sorted -> stable
	if _, err := tx.Exec(`INSERT INTO cvar_history(server_id, ts, vars) SELECT ?,?,?
		WHERE ? IS NOT (SELECT vars FROM cvar_history WHERE server_id=? ORDER BY ts DESC LIMIT 1)`,
		serverID, now, string(js), string(js), serverID); err != nil {
		return err
	}
	return tx.Commit()
}

// AddChat stores one chat line. Chat is never purged.
func (s *Store) AddChat(serverID int64, ts time.Time, guid, nameRaw, nameClean, msg string, team bool) error {
	_, err := s.conn().Exec(`INSERT INTO chat_messages(server_id, ts, guid, name_raw, name_clean, team, message) VALUES(?,?,?,?,?,?,?)`,
		serverID, ts.Unix(), guid, nameRaw, nameClean, team, msg)
	return err
}

// SeenName records that guid used this name (from the server log). Never purged.
func (s *Store) SeenName(serverID int64, ts time.Time, guid, nameRaw, nameClean string) error {
	_, err := s.conn().Exec(`INSERT INTO player_names VALUES(?,?,?,?,?,?)
		ON CONFLICT(server_id, guid, name_clean) DO UPDATE SET first_seen=MIN(first_seen, excluded.first_seen),
		  name_raw=IIF(excluded.last_seen>=last_seen, excluded.name_raw, name_raw), last_seen=MAX(last_seen, excluded.last_seen)`,
		serverID, guid, nameClean, nameRaw, ts.Unix(), ts.Unix())
	return err
}

// PlayerKey identifies a player across names: the GUID when known, else "n:<clean name>".
func PlayerKey(guid, nameClean string) string {
	if guid != "" && guid != "0" {
		return guid
	}
	return "n:" + nameClean
}

func nullIf(s string) any {
	if s == "" || s == "0" {
		return nil
	}
	return s
}

// AddKill stores one K line. An empty killer means suicide or world damage (falling...).
func (s *Store) AddKill(serverID int64, ts time.Time, kGUID, kRaw, kClean, kTeam, vGUID, vRaw, vClean, vTeam, weapon, cause, hitloc string, damage int) error {
	var killerKey any
	if kRaw != "" {
		killerKey = PlayerKey(kGUID, kClean)
	}
	teamkill := kRaw != "" && kTeam != "" && kTeam == vTeam
	_, err := s.conn().Exec(`INSERT INTO kills(server_id, ts, round_id, killer_key, killer_guid, killer_name, killer_team,
		victim_key, victim_guid, victim_name, victim_team, weapon, cause, hitloc, damage, teamkill)
		VALUES(?1, ?2, (SELECT id FROM rounds WHERE server_id=?1 AND ended_at IS NULL ORDER BY id DESC LIMIT 1),
		?3,?4,?5,?6,?7,?8,?9,?10,?11,?12,?13,?14,?15)`,
		serverID, ts.Unix(), killerKey, nullIf(kGUID), nullIf(kRaw), kTeam, PlayerKey(vGUID, vClean), nullIf(vGUID), vRaw, vTeam,
		weapon, cause, hitloc, damage, teamkill)
	return err
}

// KillExtra: details of a kill computed by the mod (KE line, logged right before its K line).
type KillExtra struct {
	Distance float64 // meters
	Tags     string  // hs,wb,ns,qs,ls,air,last,collat,triple
	Points   int
	AdsMs    int // -1 = not aiming
}

// SetLastKillExtra completes the kill just stored for this server with its KE line.
func (s *Store) SetLastKillExtra(serverID int64, x KillExtra) error {
	_, err := s.conn().Exec(`UPDATE kills SET distance=?, tags=?, points=?, ads_ms=? WHERE id=(SELECT MAX(id) FROM kills WHERE server_id=?)`,
		x.Distance, x.Tags, x.Points, x.AdsMs, serverID)
	return err
}

const openRound = `(SELECT id FROM rounds WHERE server_id=?1 AND ended_at IS NULL ORDER BY id DESC LIMIT 1)`

// SetRoundResult stores the winner and the score of the running round (RE line).
func (s *Store) SetRoundResult(serverID int64, winner string, allies, axis int, attackers string) error {
	_, err := s.conn().Exec(`UPDATE rounds SET winner=?2, score_allies=?3, score_axis=?4, attackers=?5 WHERE id=`+openRound,
		serverID, winner, allies, axis, attackers)
	return err
}

// AddGameEvent stores a game event of the running round: plant, defuse, clutch (value = enemies), hq_capture, hq_destroy, flag.
func (s *Store) AddGameEvent(serverID int64, ts time.Time, typ, guid, nameRaw, nameClean string, value int) error {
	_, err := s.conn().Exec(`INSERT INTO game_events(server_id, ts, round_id, type, key, name_raw, value) VALUES(?1,?2,`+openRound+`,?3,?4,?5,?6)`,
		serverID, ts.Unix(), typ, PlayerKey(guid, nameClean), nameRaw, value)
	return err
}

// AddMatchScore stores the final line of a player at the end of a match (MS line).
func (s *Store) AddMatchScore(serverID int64, ts time.Time, guid, nameRaw, nameClean, team string, score, kills, deaths, level, prestige int) error {
	_, err := s.conn().Exec(`INSERT INTO match_scores(server_id, ts, round_id, key, name_raw, team, score, kills, deaths, level, prestige)
		VALUES(?1,?2,(SELECT MAX(id) FROM rounds WHERE server_id=?1),?3,?4,?5,?6,?7,?8,?9,?10)`,
		serverID, ts.Unix(), PlayerKey(guid, nameClean), nameRaw, team, score, kills, deaths, level, prestige)
	return err
}

// AddMatchResult stores the winner of a match (ME line), attached to its last round.
func (s *Store) AddMatchResult(serverID int64, ts time.Time, winner string, allies, axis int) error {
	_, err := s.conn().Exec(`INSERT INTO match_results(server_id, ts, round_id, winner, score_allies, score_axis)
		VALUES(?1,?2,(SELECT MAX(id) FROM rounds WHERE server_id=?1),?3,?4,?5)`, serverID, ts.Unix(), winner, allies, axis)
	return err
}

// AddMapVote stores one card of the end-of-match map vote (MV line), or the chosen map (MVW line: chosen, no votes).
func (s *Store) AddMapVote(serverID int64, ts time.Time, mapName, mode string, votes int, chosen bool) error {
	if chosen {
		_, err := s.conn().Exec(`UPDATE map_votes SET chosen=1 WHERE id=(SELECT MAX(id) FROM map_votes WHERE server_id=? AND map=? AND mode=? AND ts>=?)`,
			serverID, mapName, mode, ts.Unix()-60)
		return err
	}
	_, err := s.conn().Exec(`INSERT INTO map_votes(server_id, ts, map, mode, votes) VALUES(?,?,?,?,?)`, serverID, ts.Unix(), mapName, mode, votes)
	return err
}

// AddReport stores a !report from the chat.
func (s *Store) AddReport(serverID int64, ts time.Time, guid, nameRaw, nameClean, target, reason string) error {
	_, err := s.conn().Exec(`INSERT INTO reports(server_id, ts, reporter_key, reporter_name, target, reason) VALUES(?,?,?,?,?,?)`,
		serverID, ts.Unix(), PlayerKey(guid, nameClean), nameRaw, target, reason)
	return err
}

// AddProfile stores the career profile a player brings (mod's PI line): level, prestige and career stats live in
// the player's own CoD4 profile, not in their GUID.
func (s *Store) AddProfile(serverID int64, ts time.Time, guid, nameRaw, nameClean, playerID string, level, prestige, kills, deaths, hs, matches int) error {
	_, err := s.conn().Exec(`INSERT INTO player_profiles(server_id, ts, key, name_raw, playerid, level, prestige, kills, deaths, headshots, matches)
		VALUES(?,?,?,?,?,?,?,?,?,?,?)`, serverID, ts.Unix(), PlayerKey(guid, nameClean), nameRaw, playerID, level, prestige, kills, deaths, hs, matches)
	return err
}

// AddFingerprint stores the network settings a client sends (rcon dumpuser): they rarely change with the GUID.
func (s *Store) AddFingerprint(serverID int64, ts time.Time, guid, nameClean string, info map[string]string) error {
	b, _ := json.Marshal(info)
	_, err := s.conn().Exec(`INSERT INTO player_fingerprints(server_id, ts, key, rate, snaps, maxpackets, userinfo) VALUES(?,?,?,?,?,?,?)`,
		serverID, ts.Unix(), PlayerKey(guid, nameClean), info["rate"], info["snaps"], info["cl_maxpackets"], string(b))
	return err
}

// Welcome returns the welcome message the admin set for this player ("" = none). A name key with a known GUID uses the GUID.
func (s *Store) Welcome(serverID int64, guid, nameClean string) string {
	var msg string
	s.conn().QueryRow(`SELECT message FROM welcome_messages WHERE server_id=? AND key=?`, serverID, PlayerKey(guid, nameClean)).Scan(&msg)
	return msg
}

// AddAccuracy stores a P_A line: shots / hits / damage since the previous line of this player (the mod resets after logging).
// AddPastKill stores a kill imported from the history of the log: no round (the rounds of that time aren't known).
func (s *Store) AddPastKill(serverID int64, ts time.Time, kGUID, kRaw, kClean, kTeam, vGUID, vRaw, vClean, vTeam, weapon, cause, hitloc string, damage int) error {
	var killerKey any
	if kRaw != "" {
		killerKey = PlayerKey(kGUID, kClean)
	}
	teamkill := kRaw != "" && kTeam != "" && kTeam == vTeam
	_, err := s.conn().Exec(`INSERT INTO kills(server_id, ts, round_id, killer_key, killer_guid, killer_name, killer_team,
		victim_key, victim_guid, victim_name, victim_team, weapon, cause, hitloc, damage, teamkill)
		VALUES(?1, ?2, NULL, ?3,?4,?5,?6,?7,?8,?9,?10,?11,?12,?13,?14,?15)`,
		serverID, ts.Unix(), killerKey, nullIf(kGUID), nullIf(kRaw), kTeam, PlayerKey(vGUID, vClean), nullIf(vGUID), vRaw, vTeam,
		weapon, cause, hitloc, damage, teamkill)
	return err
}

// Connections lists the connections of enterleave.log of a server (slot, clean name, join and leave times), to date the
// history of games_mp.log.
func (s *Store) Connections(serverID int64) (joins, leaves map[string][]int64, err error) {
	rs, err := s.DB.Query(`SELECT slot, name_clean, joined_at, left_at FROM connections WHERE server_id=?`, serverID)
	if err != nil {
		return nil, nil, err
	}
	defer rs.Close()
	joins, leaves = map[string][]int64{}, map[string][]int64{}
	for rs.Next() {
		var slot int
		var name string
		var in int64
		var out sql.NullInt64
		if err := rs.Scan(&slot, &name, &in, &out); err != nil {
			return nil, nil, err
		}
		k := strconv.Itoa(slot) + "|" + name
		joins[k] = append(joins[k], in)
		if out.Valid {
			leaves[k] = append(leaves[k], out.Int64)
		}
	}
	return joins, leaves, rs.Err()
}

// FirstGameData is the time of the oldest kill, round or chat message of a server (0: none), from the log followed live.
func (s *Store) FirstGameData(serverID int64) (int64, error) {
	var ts sql.NullInt64
	err := s.conn().QueryRow(`SELECT MIN(t) FROM (SELECT MIN(ts) AS t FROM kills WHERE server_id=?1 AND round_id IS NOT NULL
		UNION ALL SELECT MIN(started_at) FROM rounds WHERE server_id=?1 UNION ALL SELECT MIN(ts) FROM chat_messages WHERE server_id=?1)`, serverID).Scan(&ts)
	return ts.Int64, err
}

func (s *Store) AddAccuracy(serverID int64, ts time.Time, guid, nameRaw, nameClean string, shots, hits, dmgDone, dmgTaken, ffDone, ffTaken int) error {
	_, err := s.conn().Exec(`INSERT INTO accuracy(server_id, ts, round_id, key, guid, name_raw, shots, hits, dmg_done, dmg_taken, ff_done, ff_taken)
		VALUES(?1, ?2, (SELECT id FROM rounds WHERE server_id=?1 AND ended_at IS NULL ORDER BY id DESC LIMIT 1), ?3,?4,?5,?6,?7,?8,?9,?10,?11)`,
		serverID, ts.Unix(), PlayerKey(guid, nameClean), nullIf(guid), nameRaw, shots, hits, dmgDone, dmgTaken, ffDone, ffTaken)
	return err
}

// StartRound closes the running round (InitGame starts a new one, every S&D round included).
// An InitGame up to this long after a round end (RE line) belongs to the round FinishRound already opened.
const reuseRound = 120

func (s *Store) StartRound(serverID int64, ts time.Time, mapName, gametype string) error {
	var id, started int64
	var m, gt string
	err := s.conn().QueryRow(`SELECT id, started_at, COALESCE(map, ''), COALESCE(gametype, '') FROM rounds WHERE id=`+openRound, serverID).Scan(&id, &started, &m, &gt)
	if err == nil && m == mapName && gt == gametype && ts.Unix()-started <= reuseRound && s.emptyRound(id) {
		return nil // opened by the RE line of the previous round: same round
	}
	if err := s.EndRound(serverID, ts); err != nil {
		return err
	}
	_, err = s.conn().Exec(`INSERT INTO rounds(server_id, started_at, map, gametype) VALUES(?,?,?,?)`, serverID, ts.Unix(), mapName, gametype)
	return err
}

// FinishRound handles the mod's RE line: the running round gets its result and ends, and the next round of the same
// map starts (CoD4X doesn't log an InitGame for every S&D round, only when the map loads).
func (s *Store) FinishRound(serverID int64, ts time.Time, winner string, allies, axis int, attackers string) error {
	var m, gt sql.NullString
	if err := s.conn().QueryRow(`SELECT map, gametype FROM rounds WHERE id=`+openRound, serverID).Scan(&m, &gt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if err := s.SetRoundResult(serverID, winner, allies, axis, attackers); err != nil {
		return err
	}
	if _, err := s.conn().Exec(`UPDATE rounds SET ended_at=? WHERE server_id=? AND ended_at IS NULL`, ts.Unix(), serverID); err != nil {
		return err
	}
	_, err := s.conn().Exec(`INSERT INTO rounds(server_id, started_at, map, gametype) VALUES(?,?,?,?)`, serverID, ts.Unix(), m, gt)
	return err
}

// emptyRound: nothing happened in it (no kill, result, event or accuracy line).
func (s *Store) emptyRound(id int64) bool {
	var n int
	s.conn().QueryRow(`SELECT (SELECT COUNT(*) FROM kills WHERE round_id=?1) + (SELECT COUNT(*) FROM game_events WHERE round_id=?1)
		+ (SELECT COUNT(*) FROM accuracy WHERE round_id=?1) + (SELECT COUNT(*) FROM rounds WHERE id=?1 AND winner IS NOT NULL)`, id).Scan(&n)
	return n == 0
}

// EndRound closes the running round; an empty one is dropped (the round FinishRound opened after the last round of a match).
func (s *Store) EndRound(serverID int64, ts time.Time) error {
	var id int64
	if s.conn().QueryRow(`SELECT id FROM rounds WHERE id=`+openRound, serverID).Scan(&id) == nil && s.emptyRound(id) {
		_, err := s.conn().Exec(`DELETE FROM rounds WHERE id=?`, id)
		return err
	}
	_, err := s.conn().Exec(`UPDATE rounds SET ended_at=? WHERE server_id=? AND ended_at IS NULL`, ts.Unix(), serverID)
	return err
}

// ConsoleError counts an error line of the server console, grouped by message.
func (s *Store) ConsoleError(serverID int64, ts time.Time, msg string) error {
	_, err := s.conn().Exec(`INSERT INTO console_errors VALUES(?,?,1,?,?)
		ON CONFLICT(server_id, message) DO UPDATE SET count=count+1, last_seen=excluded.last_seen`, serverID, msg, ts.Unix(), ts.Unix())
	return err
}

// SetOutageCause stores what the console said before the last outage of the server.
// An empty reason doesn't overwrite one found earlier.
func (s *Store) SetOutageCause(serverID int64, reason, console string, clean bool) error {
	_, err := s.conn().Exec(`UPDATE outages SET reason=COALESCE(NULLIF(?1, ''), reason), console=COALESCE(NULLIF(?2, ''), console), clean=?3
		WHERE id=(SELECT MAX(id) FROM outages WHERE server_id=?4)`, reason, console, clean, serverID)
	return err
}

// AddRestart records a restart seen only through the server's uptime (it came back before being declared offline).
func (s *Store) AddRestart(serverID int64, from, to time.Time, reason, console string, clean bool) error {
	_, err := s.conn().Exec(`INSERT INTO outages(server_id, started_at, ended_at, reason, console, restart, clean) VALUES(?,?,?,?,?,1,?)`,
		serverID, from.Unix(), to.Unix(), reason, console, clean)
	return err
}

// LogRcon keeps an audit trail of every command sent (output truncated to 4 KB).
func (s *Store) LogRcon(serverID int64, ts time.Time, cmd, out string, cmdErr error) error {
	cmd, out = query.RedactSecrets(cmd), query.RedactSecrets(out)
	if len(out) > 4096 {
		out = out[:4096]
	}
	var e any
	if cmdErr != nil {
		e = cmdErr.Error()
	}
	_, err := s.conn().Exec(`INSERT INTO rcon_log(server_id, ts, command, output, error) VALUES(?,?,?,?,?)`, serverID, ts.Unix(), cmd, out, e)
	return err
}

func (s *Store) AddBan(serverID int64, ts time.Time, guid, nameRaw, ip, duration, reason, cmd string) error {
	_, err := s.conn().Exec(`INSERT INTO bans(server_id, ts, guid, name_raw, ip, duration, reason, command) VALUES(?,?,?,?,?,?,?,?)`,
		serverID, ts.Unix(), nullIf(guid), nameRaw, ip, duration, reason, cmd)
	return err
}

// Backup writes a consistent copy of the database to path (must not exist).
func (s *Store) Backup(path string) error {
	return s.verifiedBackup(path)
}

// LogOffset returns how far a server log file has been read, or -1 if never.
func (s *Store) LogOffset(serverID int64, file string) (int64, error) {
	var off int64 = -1
	err := s.conn().QueryRow(`SELECT pos FROM log_pos WHERE server_id=? AND file=?`, serverID, file).Scan(&off)
	if err == sql.ErrNoRows {
		err = nil
	}
	return off, err
}

func (s *Store) SetLogOffset(serverID int64, file string, off int64) error {
	_, err := s.conn().Exec(`INSERT INTO log_pos VALUES(?,?,?) ON CONFLICT(server_id, file) DO UPDATE SET pos=excluded.pos`, serverID, file, off)
	return err
}

// Enter opens a connection (enterleave.log). The GUID isn't on enter lines: guess it from the steamid or name, Leave fixes it.
func (s *Store) Enter(serverID int64, ts time.Time, slot int, addr, ip, steamID, nameRaw, nameClean string) error {
	if s.tx == nil {
		var callErr error
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		defer tx.Rollback()
		batch := &Store{DB: s.DB, key: s.key, tx: tx, writeErr: &callErr}
		if err = batch.Enter(serverID, ts, slot, addr, ip, steamID, nameRaw, nameClean); err != nil {
			return err
		}
		return tx.Commit()
	}
	// A previous connection on this slot never left. If an outage was recorded in between, it ended at the crash
	// (last successful poll before the outage); otherwise the end is unknown (NULL, not counted).
	if _, err := s.conn().Exec(`UPDATE connections SET open=0, left_at=(SELECT MAX(s.ts) FROM snapshots s
			WHERE s.server_id=?1 AND s.online=1 AND s.ts>=joined_at AND s.ts<(SELECT MIN(o.started_at) FROM outages o
			  WHERE o.server_id=?1 AND o.started_at>joined_at AND o.started_at<=?3))
		WHERE server_id=?1 AND slot=?2 AND open=1`, serverID, slot, ts.Unix()); err != nil {
		return err
	}
	if _, err := s.conn().Exec(`INSERT INTO connections(server_id, slot, addr, ip, steamid, guid, name_raw, name_clean, joined_at) VALUES(?1,?2,?3,?4,?5,
		COALESCE((SELECT guid FROM connections WHERE server_id=?1 AND ?5<>'' AND steamid=?5 AND guid IS NOT NULL ORDER BY joined_at DESC LIMIT 1),
		         (SELECT guid FROM player_names WHERE server_id=?1 AND name_clean=?7 ORDER BY last_seen DESC LIMIT 1)),
		?6,?7,?8)`, serverID, slot, addr, ip, steamID, nameRaw, nameClean, ts.Unix()); err != nil {
		return err
	}
	return nil
}

// Leave closes the matching connection and records the GUID it reveals.
func (s *Store) Leave(serverID int64, ts time.Time, slot int, addr, guid, nameRaw, nameClean string) error {
	if guid == "0" {
		guid = ""
	}
	if _, err := s.conn().Exec(`UPDATE connections SET left_at=?, open=0, guid=COALESCE(NULLIF(?, ''), guid)
		WHERE server_id=? AND slot=? AND addr=? AND open=1`, ts.Unix(), guid, serverID, slot, addr); err != nil {
		return err
	}
	if guid == "" {
		return nil
	}
	return s.SeenName(serverID, ts, guid, nameRaw, nameClean)
}

// Purge deletes data older than cutoff (chat excepted).
func (s *Store) Purge(cutoff time.Time) error {
	for _, q := range []string{
		`DELETE FROM player_sightings WHERE snapshot_id IN (SELECT id FROM snapshots WHERE ts<?)`,
		`DELETE FROM snapshots WHERE ts<?`,
		`DELETE FROM outages WHERE ended_at IS NOT NULL AND ended_at<?`,
		`DELETE FROM player_sessions WHERE open=0 AND last_seen<?`,
		`DELETE FROM cvar_history WHERE ts<? AND rowid NOT IN (SELECT MAX(rowid) FROM cvar_history GROUP BY server_id)`,
	} {
		if _, err := s.conn().Exec(q, cutoff.Unix()); err != nil {
			return err
		}
	}
	return nil
}
