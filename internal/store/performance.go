package store

const performanceSchema = `
CREATE INDEX IF NOT EXISTS kills_round ON kills(round_id, id);
CREATE INDEX IF NOT EXISTS events_round ON game_events(round_id, ts);
CREATE INDEX IF NOT EXISTS accuracy_round ON accuracy(round_id, key);
CREATE INDEX IF NOT EXISTS match_results_round ON match_results(round_id,id);
CREATE INDEX IF NOT EXISTS match_scores_round ON match_scores(round_id,key);
CREATE INDEX IF NOT EXISTS kills_killer_time ON kills(server_id,killer_key,ts);
CREATE INDEX IF NOT EXISTS kills_victim_time ON kills(server_id,victim_key,ts);
CREATE INDEX IF NOT EXISTS snapshots_latest ON snapshots(server_id,id);
CREATE INDEX IF NOT EXISTS cvars_latest ON cvar_history(server_id,ts);
CREATE INDEX IF NOT EXISTS sessions_expiry ON sessions(expires_at);
CREATE INDEX IF NOT EXISTS rounds_open ON rounds(server_id,id) WHERE ended_at IS NULL;
CREATE INDEX IF NOT EXISTS connections_time ON connections(server_id,joined_at);
CREATE TABLE IF NOT EXISTS community_goals(server_id INTEGER NOT NULL,metric TEXT NOT NULL,target INTEGER NOT NULL,PRIMARY KEY(server_id,metric));
CREATE TABLE IF NOT EXISTS community_weeks(server_id INTEGER NOT NULL,week INTEGER NOT NULL,metric TEXT NOT NULL,target INTEGER NOT NULL,PRIMARY KEY(server_id,week,metric));
CREATE TABLE IF NOT EXISTS daily_combat(server_id INTEGER NOT NULL,day INTEGER NOT NULL,key TEXT NOT NULL,
 kills INTEGER NOT NULL DEFAULT 0,deaths INTEGER NOT NULL DEFAULT 0,headshots INTEGER NOT NULL DEFAULT 0,
 PRIMARY KEY(server_id,day,key));
CREATE INDEX IF NOT EXISTS daily_combat_key ON daily_combat(server_id,key);
CREATE TRIGGER IF NOT EXISTS combat_insert AFTER INSERT ON kills BEGIN
 INSERT INTO daily_combat(server_id,day,key,kills,headshots)
 SELECT NEW.server_id,NEW.ts/86400,NEW.killer_key,1,NEW.cause='MOD_HEAD_SHOT' WHERE NEW.killer_key IS NOT NULL AND NEW.teamkill=0
 ON CONFLICT(server_id,day,key) DO UPDATE SET kills=kills+1,headshots=headshots+excluded.headshots;
 INSERT INTO daily_combat(server_id,day,key,deaths) VALUES(NEW.server_id,NEW.ts/86400,NEW.victim_key,1)
 ON CONFLICT(server_id,day,key) DO UPDATE SET deaths=deaths+1;
END;
`

// Rebuild once for existing installations; triggers maintain later inserts atomically.
func (s *Store) prepareAggregates() error {
	if s.Meta("daily_combat_version") == "1" {
		return nil
	}
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(`DELETE FROM daily_combat;
 INSERT INTO daily_combat(server_id,day,key,kills,deaths,headshots)
 SELECT server_id,day,key,SUM(k),SUM(d),SUM(h) FROM (
 SELECT server_id,ts/86400 AS day,killer_key AS key,1 AS k,0 AS d,COALESCE(cause='MOD_HEAD_SHOT',0) AS h FROM kills WHERE killer_key IS NOT NULL AND teamkill=0
 UNION ALL SELECT server_id,ts/86400,victim_key,0,1,0 FROM kills) GROUP BY server_id,day,key;
 INSERT INTO meta(key,value) VALUES('daily_combat_version','1') ON CONFLICT(key) DO UPDATE SET value='1';`); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Store) Optimize() error { _, err := s.DB.Exec(`PRAGMA optimize`); return err }
