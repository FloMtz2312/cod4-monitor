package api

import (
	"path/filepath"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestMorePlayers(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "p.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.DB.Close()
	id, _ := st.SaveServerConf(store.ServerConf{Name: "S", Host: "203.0.113.10", Port: 28960, Enabled: true})
	now := time.Now()
	// one player of enterleave.log, one only seen in games_mp.log, one too old
	if err := st.SeenName(id, now.Add(-time.Hour), "111", "^1One", "One"); err != nil {
		t.Fatal(err)
	}
	st.SeenName(id, now.Add(-2*time.Hour), "222", "Two", "Two")
	st.SeenName(id, now.AddDate(0, -3, 0), "333", "Old", "Old")
	listed := []map[string]any{{"key": "111", "guid": "111", "name_raw": "^1One", "playtime_s": int64(60)}}
	ps, err := morePlayers(st, "1", now.AddDate(0, 0, -30).Unix(), now.Unix()+1, true, listed)
	if err != nil {
		t.Fatal(err)
	}
	if len(ps) != 2 || ps[1]["key"] != "222" || ps[1]["source"] != "log" || ps[1]["playtime_s"] != nil {
		t.Fatalf("players %v", ps)
	}
}
