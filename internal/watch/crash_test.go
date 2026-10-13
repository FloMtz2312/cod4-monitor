package watch

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/mail"
	"cod4monitor/internal/push"
	"cod4monitor/internal/store"
)

func TestDiagnose(t *testing.T) {
	for _, c := range []struct {
		lines  []string
		reason string
		clean  bool
	}{
		{[]string{"InitGame", "********************", "ERROR: Exceeded limit of 1000 'xmodel' assets.", "********************", "----- Server Shutdown -----"},
			"ERROR: Exceeded limit of 1000 'xmodel' assets.", false},
		{[]string{"say: hello", "quit", "----- Server Shutdown -----", "Sys_Quit"}, "", true},
		{[]string{"Loading mp_crash", "Hitch warning: 1200 msec frame time"}, "", false},
		{[]string{"Received signal 11 (SIGSEGV)", "Segmentation fault"}, "Segmentation fault", false},
	} {
		r, clean := Diagnose(c.lines)
		if r != c.reason || clean != c.clean {
			t.Errorf("Diagnose(%q) = %q, %v; want %q, %v", c.lines, r, clean, c.reason, c.clean)
		}
	}
}

func TestCrashRestarted(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	sent := capture(t)
	old := []string{"ShutdownGame:", "ERROR: Hunk_AllocateTempMemory: failed on 1048576", "----- Server Shutdown -----"}
	c := &Crash{Mail: &mail.Mailer{}, Store: st, Server: Server{ID: 1, Name: "S"},
		ReadOld: func(context.Context) ([]string, error) { return old, nil }}
	c.Restarted(time.Unix(2000, 0), time.Unix(1985, 0))
	var reason string
	var restart int
	if err := st.DB.QueryRow(`SELECT reason, restart FROM outages`).Scan(&reason, &restart); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reason, "Hunk_") || restart != 1 {
		t.Fatalf("outage: %q %d", reason, restart)
	}
	if got := sent(); len(got) != 1 || !strings.Contains(got[0], "redémarré") {
		t.Fatalf("mails: %v", got)
	}
	// a normal shutdown (host restart) is recorded without a mail
	old = []string{"quit", "----- Server Shutdown -----"}
	c.Restarted(time.Unix(9000, 0), time.Unix(8985, 0))
	if got := sent(); len(got) != 1 {
		t.Fatalf("clean restart mailed: %v", got)
	}
}

func TestCrashAlertMail(t *testing.T) {
	c := &Crash{Mail: &mail.Mailer{}, Server: Server{Name: "Babou <Server>"}, SiteURL: "https://example.com/"}
	a := c.alert("up", "De retour en ligne", "Le serveur répond de nouveau.")
	c.explain(&a, "ERROR: Exceeded limit of 1000 'xmodel' assets.", false,
		[]string{"^1InitGame", "ERROR: Exceeded limit of 1000 'xmodel' assets.", "----- Server Shutdown -----"}, true)
	text, htm := a.render()
	for _, want := range []string{"Babou <Server> — De retour en ligne", "Cause probable", "> ERROR: Exceeded limit", "  InitGame", "https://example.com/"} {
		if !strings.Contains(text, want) {
			t.Errorf("text misses %q:\n%s", want, text)
		}
	}
	for _, want := range []string{"Babou &lt;Server&gt;", "#2b8a3e", "font-weight:700\">ERROR: Exceeded limit", "Une limite du moteur", "href=\"https://example.com/\""} {
		if !strings.Contains(htm, want) {
			t.Errorf("html misses %q", want)
		}
	}
}

type pushService chan *http.Request

func (p pushService) RoundTrip(r *http.Request) (*http.Response, error) {
	p <- r
	return &http.Response{StatusCode: 201, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func TestCrashPush(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	ps, err := push.New(st, "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	got := make(pushService, 4)
	ps.Client = &http.Client{Transport: got}
	k, _ := ecdh.P256().GenerateKey(rand.Reader)
	b64 := base64.RawURLEncoding
	st.SavePushSub(store.PushSub{Endpoint: "https://web.push.apple.com/x", P256dh: b64.EncodeToString(k.PublicKey().Bytes()),
		Auth: b64.EncodeToString(make([]byte, 16)), Types: []string{"down"}}, time.Now())
	c := &Crash{Store: st, Server: Server{ID: 1, Name: "^1S"}, Push: ps} // no mail: the push goes anyway
	c.Down(time.Unix(2000, 0), time.Unix(1950, 0))
	select {
	case r := <-got:
		if r.Header.Get("Topic") != "down-1" || r.Header.Get("Urgency") != "high" {
			t.Fatalf("push headers %v", r.Header)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push for the server going down")
	}
}
