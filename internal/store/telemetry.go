package store

import "time"

const telemetrySchema = `
CREATE TABLE IF NOT EXISTS log_health(server_id INTEGER NOT NULL,file TEXT NOT NULL,
 last_success INTEGER NOT NULL DEFAULT 0,backlog_bytes INTEGER NOT NULL DEFAULT 0,
 lines INTEGER NOT NULL DEFAULT 0,rejected INTEGER NOT NULL DEFAULT 0,errors INTEGER NOT NULL DEFAULT 0,
 rotations INTEGER NOT NULL DEFAULT 0,missing INTEGER NOT NULL DEFAULT 0,PRIMARY KEY(server_id,file));
`

func (s *Store) LogHealthRead(id int64, file string, backlog int64, lines, rejected int) error {
	_, err := s.conn().Exec(`INSERT INTO log_health(server_id,file,last_success,backlog_bytes,lines,rejected,missing) VALUES(?,?,?,?,?,?,0)
 ON CONFLICT(server_id,file) DO UPDATE SET last_success=excluded.last_success,backlog_bytes=excluded.backlog_bytes,
 lines=log_health.lines+excluded.lines,rejected=log_health.rejected+excluded.rejected,missing=0`, id, file, time.Now().Unix(), backlog, lines, rejected)
	return err
}
func (s *Store) LogHealthSignal(id int64, file, signal string) error {
	col := map[string]string{"error": "errors", "rotation": "rotations", "missing": "missing"}[signal]
	if col == "" {
		return nil
	}
	value := col + "+1"
	if signal == "missing" {
		value = "1"
	}
	_, err := s.conn().Exec(`INSERT INTO log_health(server_id,file,`+col+`) VALUES(?,?,1) ON CONFLICT(server_id,file) DO UPDATE SET `+col+`=`+value, id, file)
	return err
}
