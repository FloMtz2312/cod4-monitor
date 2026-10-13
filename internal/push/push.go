// Package push sends Web Push notifications to the phones and browsers of the staff: server down or back, new
// report, suspect or banned player back in game, B3 ban. Standard Web Push (RFC 8030) with the payload encrypted for
// the browser (RFC 8291, aes128gcm) and the monitor identified by a VAPID key (RFC 8292), created on the first start
// and kept in the database. No third-party service: the browser's own push service (Google, Apple, Mozilla,
// Microsoft) delivers it, even with the app closed.
package push

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"cod4monitor/internal/store"
)

// Kinds of notifications a subscriber chooses from.
var Kinds = []string{"down", "report", "suspect", "b3ban"}

// AppPath is where the app opens when a notification is tapped.
const AppPath = "/"

// ErrGone: the browser unsubscribed or the app was removed; it has been forgotten.
var ErrGone = errors.New("subscription gone")

type Sender struct {
	Store   *store.Store
	Subject string // VAPID contact: the dashboard's https URL or a mailto:
	// Access: whether the account owns the server, else its permissions there. nil = everyone may see everything.
	Access func(user string, server int64) (owner bool, perms map[string]bool)
	Client *http.Client

	key *ecdsa.PrivateKey
	pub string // uncompressed public key, base64url: the applicationServerKey of the browsers
}

// New loads the VAPID key of the monitor, or creates it the first time. Changing it would invalidate every
// subscription, so it lives in the database (and its backups).
func New(st *store.Store, subject string) (*Sender, error) {
	var key *ecdsa.PrivateKey
	if raw, err := b64.DecodeString(st.Meta("vapid_private")); err == nil && len(raw) == 32 {
		if key, err = ecdsa.ParseRawPrivateKey(elliptic.P256(), raw); err != nil {
			return nil, err
		}
	} else {
		if key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader); err != nil {
			return nil, err
		}
		raw, err := key.Bytes()
		if err != nil {
			return nil, err
		}
		if err := st.SetMeta("vapid_private", b64.EncodeToString(raw)); err != nil {
			return nil, err
		}
	}
	pub, err := key.PublicKey.Bytes()
	if err != nil {
		return nil, err
	}
	return &Sender{Store: st, Subject: subject, key: key, pub: b64.EncodeToString(pub)}, nil
}

// PublicKey for pushManager.subscribe (nil-safe: "" when push is off).
func (s *Sender) PublicKey() string {
	if s == nil {
		return ""
	}
	return s.pub
}

// Message: one notification. The text is chosen by the language of each subscriber.
type Message struct {
	Kind            string   // one of Kinds: only to the subscribers who chose it ("test": to the one browser)
	Server          int64    // server concerned: muted servers and permissions are checked against it
	Need            []string // one of these permissions is needed on the server (an owner has them all); none = any access
	Title, Body     string   // French
	TitleEN, BodyEN string   // English ("" = the French text)
	Path            string   // screen of the app opened by a tap: "moderation", "etat"... ("" = the live screen)
	Tag             string   // a notification with the same tag replaces the previous one on the phone
	Urgent          bool     // delivered at once even on a phone saving battery
}

func (m Message) text(lang string) (title, body string) {
	title, body = m.Title, m.Body
	if lang == "en" {
		if m.TitleEN != "" {
			title = m.TitleEN
		}
		if m.BodyEN != "" {
			body = m.BodyEN
		}
	}
	return title, cut(body, 600)
}

func cut(s string, n int) string {
	if r := []rune(s); len(r) > n {
		return string(r[:n-1]) + "…"
	}
	return s
}

// payload read by the service worker (ui/public/sw.js).
func (m Message) payload(lang string) []byte {
	title, body := m.text(lang)
	u := AppPath
	if m.Server > 0 {
		u += "?s=" + strconv.FormatInt(m.Server, 10)
	}
	u += "#/" + m.Path
	b, _ := json.Marshal(map[string]any{"title": title, "body": body, "url": u, "tag": m.Tag, "server": m.Server})
	return b
}

// Allowed: the subscriber wants this kind, didn't mute the server, and has the rights to see it.
func (s *Sender) Allowed(sub store.PushSub, m Message) bool {
	if !slices.Contains(sub.Types, m.Kind) || slices.Contains(sub.Muted, m.Server) {
		return false
	}
	if s.Access == nil {
		return true
	}
	owner, perms := s.Access(sub.Username, m.Server)
	if owner {
		return true
	}
	if len(m.Need) == 0 {
		for _, ok := range perms {
			if ok {
				return true
			}
		}
		return false
	}
	for _, p := range m.Need {
		if perms[p] {
			return true
		}
	}
	return false
}

// Notify sends the message, in the background, to every subscriber who should get it.
func (s *Sender) Notify(m Message) {
	if s == nil {
		return
	}
	go s.notify(m)
}

func (s *Sender) notify(m Message) (sent int) {
	subs, err := s.Store.PushSubs()
	if err != nil {
		slog.Error("push subscribers", "err", err)
		return 0
	}
	for _, sub := range subs {
		if !s.Allowed(sub, m) {
			continue
		}
		if err := s.Send(sub, m); err != nil {
			if !errors.Is(err, ErrGone) {
				slog.Warn("push", "kind", m.Kind, "user", sub.Username, "service", host(sub.Endpoint), "err", err)
			}
			continue
		}
		sent++
	}
	return sent
}

// Send delivers one message to one browser. A browser the push service no longer knows is forgotten (ErrGone).
func (s *Sender) Send(sub store.PushSub, m Message) error {
	if !ValidEndpoint(sub.Endpoint) {
		return errors.New("invalid endpoint")
	}
	uaPub, err1 := b64.DecodeString(sub.P256dh)
	auth, err2 := b64.DecodeString(sub.Auth)
	if err1 != nil || err2 != nil {
		return errors.New("invalid browser keys")
	}
	as, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	rand.Read(salt)
	body, err := encrypt(m.payload(sub.Lang), uaPub, auth, as, salt)
	if err != nil {
		return err
	}
	now := time.Now()
	vapid, err := s.vapid(sub.Endpoint, now)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("Authorization", vapid)
	req.Header.Set("TTL", "43200") // a phone off for the night still gets it in the morning, not later
	req.Header.Set("Urgency", map[bool]string{true: "high", false: "normal"}[m.Urgent])
	if m.Tag != "" {
		req.Header.Set("Topic", topic(m.Tag)) // replaces the previous one still waiting (server down, then back)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		s.Store.PushResult(sub.Endpoint, false, now)
		return err
	}
	defer resp.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(resp.Body, 300))
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return s.Store.PushResult(sub.Endpoint, true, now)
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		s.Store.DeletePushSub(sub.Endpoint, "")
		return ErrGone
	}
	s.Store.PushResult(sub.Endpoint, false, now)
	return fmt.Errorf("push service: %s %s", resp.Status, strings.TrimSpace(string(msg)))
}

func (s *Sender) client() *http.Client {
	if s.Client != nil {
		return s.Client
	}
	return &http.Client{Timeout: 15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
}

var b64 = base64.RawURLEncoding

// pushHosts: the push services of the browsers. The monitor only posts to them (the endpoint comes from the browser).
var pushHosts = []string{"googleapis.com", "push.apple.com", "mozilla.com", "notify.windows.com"}

// ValidEndpoint: an https URL of a known push service.
func ValidEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || len(endpoint) > 1000 {
		return false
	}
	h := strings.ToLower(u.Hostname())
	for _, p := range pushHosts {
		if h == p || strings.HasSuffix(h, "."+p) {
			return true
		}
	}
	return false
}

func host(endpoint string) string {
	if u, err := url.Parse(endpoint); err == nil {
		return u.Hostname()
	}
	return ""
}

// topic: at most 32 characters of the base64url alphabet.
func topic(tag string) string {
	t := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			return r
		}
		return '_'
	}, tag)
	if len(t) > 32 {
		t = t[:32]
	}
	return t
}

// vapid: the Authorization header, a JWT signed with the monitor's key for the push service of the endpoint.
func (s *Sender) vapid(endpoint string, now time.Time) (string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	claims, _ := json.Marshal(map[string]any{"aud": u.Scheme + "://" + u.Host, "exp": now.Add(12 * time.Hour).Unix(), "sub": s.Subject})
	unsigned := b64.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + b64.EncodeToString(claims)
	h := sha256.Sum256([]byte(unsigned))
	r, sg, err := ecdsa.Sign(rand.Reader, s.key, h[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64) // JWS: r and s, 32 bytes each
	r.FillBytes(sig[:32])
	sg.FillBytes(sig[32:])
	return "vapid t=" + unsigned + "." + b64.EncodeToString(sig) + ", k=" + s.pub, nil
}

// encrypt (RFC 8291): one aes128gcm record, readable only by the browser that holds the private key of uaPub.
func encrypt(plain, uaPub, authSecret []byte, as *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	ua, err := ecdh.P256().NewPublicKey(uaPub)
	if err != nil {
		return nil, err
	}
	if len(authSecret) != 16 || len(salt) != 16 {
		return nil, errors.New("invalid auth secret or salt")
	}
	secret, err := as.ECDH(ua)
	if err != nil {
		return nil, err
	}
	asPub := as.PublicKey().Bytes()
	ikm, err := hkdf.Key(sha256.New, secret, authSecret, "WebPush: info\x00"+string(uaPub)+string(asPub), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(plain) > 3800 {
		return nil, errors.New("payload too large")
	}
	out := append([]byte{}, salt...)
	out = binary.BigEndian.AppendUint32(out, 4096) // record size
	out = append(out, byte(len(asPub)))
	out = append(out, asPub...)
	return gcm.Seal(out, nonce, append(append([]byte{}, plain...), 2), nil), nil // 2: last record, no padding
}
