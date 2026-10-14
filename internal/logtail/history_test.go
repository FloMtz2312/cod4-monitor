package logtail

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestImportHistory(t *testing.T) {
	tl, st, _ := recoveryFixture(t)
	t0 := time.Date(2026, 9, 1, 20, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	// enterleave.log: Alpha and Bravo join in the first run of the server, Bravo leaves; Delta joins much later
	st.Enter(1, at(30), 0, "1.1.1.1:1", "1.1.1.1", "", "Alpha", "Alpha")
	st.Enter(1, at(60), 1, "2.2.2.2:1", "2.2.2.2", "", "Bravo", "Bravo")
	st.Leave(1, at(180), 1, "2.2.2.2:1", "222", "Bravo", "Bravo")
	st.Enter(1, at(20000), 0, "4.4.4.4:1", "4.4.4.4", "", "Delta", "Delta")
	// the log followed live from t0+10000: its first round
	st.StartRound(1, at(10000), "mp_crash", "sd")

	games := strings.Join([]string{
		// run 1: dated by the joins and the quit
		"  0:00 InitGame: \\mapname\\mp_crash\\g_gametype\\sd",
		"  0:30 J;111;0;^1Alpha",
		"  1:00 J;222;1;Bravo",
		"  2:00 K;222;1;axis;Bravo;111;0;allies;^1Alpha;m40a3_mp;100;MOD_HEAD_SHOT;head",
		"  2:10 say;111;0;^1Alpha;gg",
		"  3:00 Q;222;1;Bravo",
		// run 2: no connection in enterleave.log, undated
		"  0:00 InitGame: \\mapname\\mp_crash\\g_gametype\\sd",
		"  0:10 J;333;2;Charlie",
		// run 3: after the live start, already read
		"  0:00 InitGame: \\mapname\\mp_crash\\g_gametype\\sd",
		"  5:00 J;444;0;Delta",
		"  6:00 J;555;3;Echo",
	}, "\n") + "\n"
	os.WriteFile(filepath.Join(strings.TrimPrefix(tl.Dir.Path, ""), "games_mp.log"), []byte(games), 0o644)
	// run 3 gets dated through Delta and a second line
	st.Enter(1, at(20060), 3, "5.5.5.5:1", "5.5.5.5", "", "Echo", "Echo")

	tl.importHistory(context.Background(), st)

	var first, last int64
	if err := st.DB.QueryRow(`SELECT first_seen, last_seen FROM player_names WHERE guid='111'`).Scan(&first, &last); err != nil {
		t.Fatal(err)
	}
	if first != at(30).Unix() || last != at(130).Unix() {
		t.Fatalf("Alpha seen %v → %v", time.Unix(first, 0).UTC(), time.Unix(last, 0).UTC())
	}
	var n int
	st.DB.QueryRow(`SELECT COUNT(*) FROM player_names WHERE guid IN ('333', '444', '555')`).Scan(&n)
	if n != 0 {
		t.Fatalf("undated or live players imported: %d", n)
	}
	var ts int64
	var round any
	if err := st.DB.QueryRow(`SELECT ts, round_id FROM kills WHERE killer_key='111'`).Scan(&ts, &round); err != nil || ts != at(120).Unix() || round != nil {
		t.Fatalf("kill at %v round %v: %v", time.Unix(ts, 0).UTC(), round, err)
	}
	st.DB.QueryRow(`SELECT COUNT(*) FROM chat_messages WHERE message='gg'`).Scan(&n)
	if n != 1 {
		t.Fatalf("chat %d", n)
	}
	// once only
	tl.importHistory(context.Background(), st)
	st.DB.QueryRow(`SELECT COUNT(*) FROM kills`).Scan(&n)
	if n != 1 {
		t.Fatalf("imported twice: %d kills", n)
	}
}
