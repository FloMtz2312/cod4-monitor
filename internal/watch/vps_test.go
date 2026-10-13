package watch

import (
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cod4monitor/internal/mail"
	"cod4monitor/internal/store"
)

// capture replaces sendMail and returns the subjects sent (send() is asynchronous: wait a little).
func capture(t *testing.T) func() []string {
	var mu sync.Mutex
	var got []string
	old := sendMail
	sendMail = func(m *mail.Mailer, subject, text, html string) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, subject)
		return nil
	}
	t.Cleanup(func() { sendMail = old })
	return func() []string {
		time.Sleep(50 * time.Millisecond)
		mu.Lock()
		defer mu.Unlock()
		return append([]string{}, got...)
	}
}

func TestVPSCheck(t *testing.T) {
	sent := capture(t)
	v := &VPS{Mail: &mail.Mailer{}, DiskPct: 90, MemPct: 90, Load: 2}
	now := time.Unix(1_000_000, 0)
	ok := Reading{50, 50, 0.5}
	v.Check(Reading{95, 50, 0.5}, now) // disk: at once
	for i := 0; i < sustain-1; i++ {   // memory: only after 5 checks in a row
		v.Check(Reading{95, 95, 0.5}, now.Add(time.Duration(i+1)*time.Minute))
	}
	if got := sent(); len(got) != 1 || !strings.Contains(got[0], "Disque") {
		t.Fatalf("after 4 checks: %v", got)
	}
	v.Check(Reading{95, 95, 0.5}, now.Add(5*time.Minute))
	v.Check(Reading{95, 95, 0.5}, now.Add(6*time.Minute)) // still bad: no new mail
	if got := strings.Join(sent(), "|"); strings.Count(got, "⚠") != 2 || !strings.Contains(got, "Mémoire") {
		t.Fatalf("memory: %v", got)
	}
	v.Check(Reading{95, 50, 0.5}, now.Add(25*time.Hour)) // disk reminder, memory over
	v.Check(ok, now.Add(26*time.Hour))
	got := strings.Join(sent(), "|") // sent in the background: any order
	if strings.Count(got, "⚠ Disque") != 2 || strings.Count(got, "⚠ Mémoire") != 1 || strings.Count(got, "✓ Mémoire") != 1 || strings.Count(got, "✓ Disque") != 1 {
		t.Fatalf("reminder and recoveries: %v", got)
	}
}

func TestStarted(t *testing.T) {
	sent := capture(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	m := &mail.Mailer{Host: "smtp.example.com", To: []string{"a@example.com"}}
	now := time.Unix(1_000_000, 0)
	Started(st, m, "", now) // first start: the "mails work" message
	Started(st, m, "", now.Add(time.Minute))
	if got := sent(); len(got) != 1 || !strings.Contains(got[0], "configurés") {
		t.Fatalf("setup mail: %v", got)
	}
	Started(st, m, "", now.Add(2*time.Minute)) // 3rd start in 15 min
	Started(st, m, "", now.Add(3*time.Minute)) // within the hour: no second alert
	if got := sent(); len(got) != 2 || !strings.Contains(got[1], "boucle") {
		t.Fatalf("restart loop: %v", got)
	}
}

func TestRead(t *testing.T) {
	r := Read(t.TempDir())
	if r.DiskPct < 0 || r.DiskPct > 100 || r.MemPct < 0 || r.LoadPerCPU < 0 {
		t.Fatalf("reading: %+v", r)
	}
}

func TestWeekly(t *testing.T) {
	sent := capture(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	st.UpsertServer("S", "127.0.0.1", 28960)
	loc := time.UTC
	from, _ := time.Date(2026, 9, 28, 0, 0, 0, 0, loc), 0
	st.StartRound(1, from.Add(time.Hour), "mp_crash", "sd")
	st.AddKill(1, from.Add(time.Hour+time.Minute), "g1", "^1Alice<b>", "Alice<b>", "axis", "g2", "Bob", "Bob", "allies", "m40a3_mp", "MOD_HEAD_SHOT", "head", 100)
	w := &Weekly{Mail: &mail.Mailer{}, Store: st, Servers: []Server{{1, "Mon Serveur"}}, Loc: loc, SiteURL: "https://x.duckdns.org/"}

	w.tick(time.Date(2026, 10, 5, 8, 0, 0, 0, loc)) // Monday before 9:00: nothing
	if got := sent(); len(got) != 0 {
		t.Fatalf("before 9: %v", got)
	}
	w.tick(time.Date(2026, 10, 5, 9, 5, 0, 0, loc))
	w.tick(time.Date(2026, 10, 5, 9, 15, 0, 0, loc)) // once
	w.tick(time.Date(2026, 10, 8, 9, 15, 0, 0, loc)) // Thursday: same week already sent
	if got := sent(); len(got) != 1 || got[0] != "📊 Résumé de la semaine du 28/09 au 04/10" {
		t.Fatalf("weekly: %v", got)
	}
	_, text, htm, err := w.Build(from, from.AddDate(0, 0, 7))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text, "Joueur de la semaine : Alice<b> (1 kills") || !strings.Contains(htm, "Alice&lt;b&gt;") || strings.Contains(htm, "Alice<b>") {
		t.Fatalf("content:\n%s\n%s", text, htm)
	}
}

func TestPlayersJoined(t *testing.T) {
	sent := capture(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(10_000_000, 0)
	st.DB.Exec(`INSERT INTO player_notes(server_id, key, tag, note) VALUES(1, 'guidcheat', 'suspect', 'aimbot ?'), (1, 'guidvip', 'vip', '')`)
	st.AddReport(1, now.Add(-time.Hour), "g9", "Rep", "Rep", "wall", "wallhack")
	st.AddReport(1, now.Add(-40*24*time.Hour), "g9", "Rep", "Rep", "oldguy", "too old")
	p := &Players{Mail: &mail.Mailer{}, DB: st.DB, SiteURL: "https://x/"}
	s := Server{1, "Babou"}
	p.Joined(s, "guidvip", "^2Vip", now)                    // vip tag: no alert
	p.Joined(s, "g3", "oldguy", now)                        // report older than 30 days
	p.Joined(s, "guidcheat", "^1Cheat", now)                // suspect note
	p.Joined(s, "guidcheat", "^1Cheat", now.Add(time.Hour)) // cooldown
	p.Joined(s, "g4", "^3WallE", now)                       // matches the report "wall"
	got := strings.Join(sent(), "|")
	if strings.Count(got, "vient de se connecter") != 2 || !strings.Contains(got, "Cheat") || !strings.Contains(got, "WallE") {
		t.Fatalf("alerts: %v", got)
	}
}

func TestVPSAlertMail(t *testing.T) {
	var mu sync.Mutex
	var htm []string
	old := sendMail
	sendMail = func(m *mail.Mailer, subject, text, html string) error {
		mu.Lock()
		defer mu.Unlock()
		htm = append(htm, html)
		return nil
	}
	t.Cleanup(func() { sendMail = old })
	v := &VPS{Mail: &mail.Mailer{}, DiskPct: 90, MemPct: 90, Load: 2, SiteURL: "https://x/"}
	v.Check(Reading{95, 50, 0.5}, time.Unix(1_000_000, 0))
	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	defer mu.Unlock()
	if len(htm) != 1 {
		t.Fatalf("mails: %d", len(htm))
	}
	for _, want := range []string{"Disque presque plein", "95 % (alerte à 90 %)", "docker system df", "#f08c00", `href="https://x/"`} {
		if !strings.Contains(htm[0], want) {
			t.Errorf("html misses %q", want)
		}
	}
}
