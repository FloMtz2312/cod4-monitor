package push

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := b64.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The example of RFC 8291, appendix A.
func TestEncryptRFC8291(t *testing.T) {
	as, err := ecdh.P256().NewPrivateKey(mustB64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	uaPub := mustB64(t, "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4")
	got, err := encrypt([]byte("When I grow up, I want to be a watermelon"), uaPub, mustB64(t, "BTBZMqHH6r4Tts7J_aSIgg"), as,
		mustB64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	want := "DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z9KsN6nGRTbVYI_c7VJSPQTBtkgcy27mlmlMoZIIgDll6e3vCYLocInmYWAmS6TlzAC8wEqKK6PBru3jl7A_yl95bQpu6cVPTpK4Mqgkf1CXztLVBSt2Ks3oZwbuwXPXLWyouBWLVWGNWQexSgSxsj_Qulcy4a-fN"
	if b64.EncodeToString(got) != want {
		t.Fatalf("encrypt = %s\nwant      %s", b64.EncodeToString(got), want)
	}
}

// browser: what a browser does with a push, to check the whole message.
type browser struct {
	key  *ecdh.PrivateKey
	auth []byte
}

func newBrowser(t *testing.T) browser {
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	rand.Read(auth)
	return browser{k, auth}
}

func (b browser) decrypt(t *testing.T, body []byte) []byte {
	t.Helper()
	salt, rs, idlen := body[:16], binary.BigEndian.Uint32(body[16:20]), int(body[20])
	asPub, ct := body[21:21+idlen], body[21+idlen:]
	if rs != 4096 || idlen != 65 {
		t.Fatalf("header rs=%d idlen=%d", rs, idlen)
	}
	pub, err := ecdh.P256().NewPublicKey(asPub)
	if err != nil {
		t.Fatal(err)
	}
	secret, _ := b.key.ECDH(pub)
	ikm, _ := hkdf.Key(sha256.New, secret, b.auth, "WebPush: info\x00"+string(b.key.PublicKey().Bytes())+string(asPub), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plain[len(plain)-1] != 2 {
		t.Fatalf("padding delimiter %d", plain[len(plain)-1])
	}
	return plain[:len(plain)-1]
}

// service: a fake push service that records the requests.
type service struct {
	mu     sync.Mutex
	status int
	reqs   []*http.Request
	bodies [][]byte
}

func (s *service) RoundTrip(r *http.Request) (*http.Response, error) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reqs, s.bodies = append(s.reqs, r), append(s.bodies, b)
	st := s.status
	if st == 0 {
		st = 201
	}
	return &http.Response{StatusCode: st, Status: http.StatusText(st), Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
}

func setup(t *testing.T) (*Sender, *service, *store.Store) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.DB.Close() })
	s, err := New(st, "https://monitor.example")
	if err != nil {
		t.Fatal(err)
	}
	svc := &service{}
	s.Client = &http.Client{Transport: svc}
	return s, svc, st
}

func TestKeyKept(t *testing.T) {
	s, _, st := setup(t)
	again, err := New(st, "")
	if err != nil {
		t.Fatal(err)
	}
	if s.PublicKey() == "" || again.PublicKey() != s.PublicKey() || len(mustB64(t, s.PublicKey())) != 65 {
		t.Fatalf("key %q then %q", s.PublicKey(), again.PublicKey())
	}
}

func TestSend(t *testing.T) {
	s, svc, st := setup(t)
	b := newBrowser(t)
	sub := store.PushSub{Endpoint: "https://fcm.googleapis.com/fcm/send/abc", Username: "werner", P256dh: b64.EncodeToString(b.key.PublicKey().Bytes()),
		Auth: b64.EncodeToString(b.auth), Lang: "en", Types: []string{"report"}}
	if err := st.SavePushSub(sub, time.Now()); err != nil {
		t.Fatal(err)
	}
	s.Access = func(user string, server int64) (bool, map[string]bool) {
		return false, map[string]bool{"reports": user == "werner" && server == 1}
	}
	m := Message{Kind: "report", Server: 1, Need: []string{"reports"}, Title: "Signalement", TitleEN: "Report", Body: "a signale b",
		Path: "moderation", Tag: "report-1", Urgent: true}
	if n := s.notify(m); n != 1 {
		t.Fatalf("sent %d", n)
	}
	r := svc.reqs[0]
	if r.URL.String() != sub.Endpoint || r.Header.Get("Content-Encoding") != "aes128gcm" || r.Header.Get("Urgency") != "high" ||
		r.Header.Get("Topic") != "report-1" || r.Header.Get("TTL") == "" {
		t.Fatalf("request %s %v", r.URL, r.Header)
	}
	var p map[string]any
	if err := json.Unmarshal(b.decrypt(t, svc.bodies[0]), &p); err != nil {
		t.Fatal(err)
	}
	if p["title"] != "Report" || p["body"] != "a signale b" || p["url"] != "/?s=1#/moderation" || p["tag"] != "report-1" {
		t.Fatalf("payload %v", p)
	}

	// the VAPID token: signed by the key the browsers know, for this push service
	auth := r.Header.Get("Authorization")
	tok, k, ok := strings.Cut(strings.TrimPrefix(auth, "vapid t="), ", k=")
	if !ok || k != s.PublicKey() {
		t.Fatalf("authorization %q", auth)
	}
	parts := strings.Split(tok, ".")
	var claims struct {
		Aud, Sub string
		Exp      int64
	}
	json.Unmarshal(mustB64(t, parts[1]), &claims)
	if claims.Aud != "https://fcm.googleapis.com" || claims.Sub != "https://monitor.example" || claims.Exp <= time.Now().Unix() {
		t.Fatalf("claims %+v", claims)
	}
	pub, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), mustB64(t, k))
	if err != nil {
		t.Fatal(err)
	}
	sig, h := mustB64(t, parts[2]), sha256.Sum256([]byte(parts[0]+"."+parts[1]))
	if !ecdsa.Verify(pub, h[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("bad VAPID signature")
	}

	// not for those without the permission, who didn't choose the kind, or muted the server
	for _, m2 := range []Message{{Kind: "report", Server: 2, Need: []string{"reports"}}, {Kind: "down", Server: 1}, {Kind: "report", Server: 1, Need: []string{"private"}}} {
		if n := s.notify(m2); n != 0 {
			t.Fatalf("%+v sent to %d", m2, n)
		}
	}
	sub.Types, sub.Muted = []string{"report", "down"}, []int64{1}
	st.SavePushSub(sub, time.Now())
	if n := s.notify(m); n != 0 {
		t.Fatal("muted server notified")
	}

	// the push service forgot the browser: so does the monitor
	sub.Muted = nil
	st.SavePushSub(sub, time.Now())
	svc.status = 410
	if n := s.notify(m); n != 0 {
		t.Fatal("gone counted as sent")
	}
	if _, ok, _ := st.PushSub(sub.Endpoint); ok {
		t.Fatal("gone subscription kept")
	}
}

func TestAnyAccess(t *testing.T) {
	s, _, _ := setup(t)
	sub := store.PushSub{Username: "mod", Types: Kinds}
	s.Access = func(user string, server int64) (bool, map[string]bool) {
		if server == 1 {
			return false, map[string]bool{"say": true}
		}
		return false, map[string]bool{}
	}
	if !s.Allowed(sub, Message{Kind: "down", Server: 1}) || s.Allowed(sub, Message{Kind: "down", Server: 2}) {
		t.Fatal("server down: for anyone of its staff only")
	}
	s.Access = func(string, int64) (bool, map[string]bool) { return true, nil }
	if !s.Allowed(sub, Message{Kind: "b3ban", Server: 2, Need: []string{"private"}}) {
		t.Fatal("an owner gets everything")
	}
}

func TestValidEndpoint(t *testing.T) {
	for e, want := range map[string]bool{
		"https://fcm.googleapis.com/fcm/send/x":                true,
		"https://web.push.apple.com/QGx":                       true,
		"https://updates.push.services.mozilla.com/wpush/v2/x": true,
		"https://wns2-par02p.notify.windows.com/w/?token=x":    true,
		"http://fcm.googleapis.com/fcm/send/x":                 false,
		"https://127.0.0.1/x":                                  false,
		"https://evilgoogleapis.com/x":                         false,
		"https://fcm.googleapis.com:8443/x":                    false,
		"https://user@fcm.googleapis.com/x":                    false,
	} {
		if ValidEndpoint(e) != want {
			t.Errorf("ValidEndpoint(%s) = %v", e, !want)
		}
	}
}
