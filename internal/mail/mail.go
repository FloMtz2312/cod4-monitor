// Package mail sends the monitor's emails (VPS alerts, weekly summary, player alerts) through an SMTP server.
package mail

import (
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"strings"
	"time"
)

// Mailer is nil-safe: a nil *Mailer (no SMTP configured) sends nothing.
type Mailer struct {
	Host, User, Pass, From string
	Port                   int
	To                     []string
	Lang                   string // "fr" (default) or "en", from MAIL_LANG
}

// T picks the French or English text, by MAIL_LANG.
func (m *Mailer) T(fr, en string) string {
	if m != nil && m.Lang == "en" {
		return en
	}
	return fr
}

// FromEnv reads SMTP_HOST, SMTP_PORT (587), SMTP_USER, SMTP_PASSWORD, MAIL_FROM (default SMTP_USER), MAIL_TO
// (comma separated) and MAIL_LANG. Returns nil when SMTP_HOST or MAIL_TO is empty.
func FromEnv() *Mailer {
	host, to := os.Getenv("SMTP_HOST"), os.Getenv("MAIL_TO")
	if host == "" || to == "" {
		return nil
	}
	port, err := strconv.Atoi(os.Getenv("SMTP_PORT"))
	if err != nil || port == 0 {
		port = 587
	}
	m := &Mailer{Host: host, Port: port, User: os.Getenv("SMTP_USER"), Pass: os.Getenv("SMTP_PASSWORD"), From: os.Getenv("MAIL_FROM"),
		Lang: strings.ToLower(os.Getenv("MAIL_LANG"))}
	if m.From == "" {
		m.From = m.User
	}
	for _, a := range strings.Split(to, ",") {
		if a = strings.TrimSpace(a); a != "" {
			m.To = append(m.To, a)
		}
	}
	return m
}

// Message builds the raw email: a plain text part and, if html is not empty, an HTML part.
func (m *Mailer) Message(subject, text, html string, now time.Time) []byte {
	var b strings.Builder
	h := func(k, v string) { fmt.Fprintf(&b, "%s: %s\r\n", k, v) }
	h("From", mime.QEncoding.Encode("utf-8", "CoD4 Monitor")+" <"+m.From+">")
	h("To", strings.Join(m.To, ", "))
	h("Subject", mime.QEncoding.Encode("utf-8", subject))
	h("Date", now.Format(time.RFC1123Z))
	h("MIME-Version", "1.0")
	part := func(ctype, body string) {
		h("Content-Type", ctype+"; charset=utf-8")
		h("Content-Transfer-Encoding", "base64")
		b.WriteString("\r\n")
		enc := base64.StdEncoding.EncodeToString([]byte(body))
		for len(enc) > 76 {
			b.WriteString(enc[:76] + "\r\n")
			enc = enc[76:]
		}
		b.WriteString(enc + "\r\n")
	}
	if html == "" {
		part("text/plain", text)
		return []byte(b.String())
	}
	const boundary = "cod4monitor-alt"
	h("Content-Type", `multipart/alternative; boundary="`+boundary+`"`)
	b.WriteString("\r\n--" + boundary + "\r\n")
	part("text/plain", text)
	b.WriteString("--" + boundary + "\r\n")
	part("text/html", html)
	b.WriteString("--" + boundary + "--\r\n")
	return []byte(b.String())
}

// Send delivers the email. Port 465 uses TLS from the start, other ports STARTTLS when the server offers it.
func (m *Mailer) Send(subject, text, html string) error {
	if m == nil {
		return nil
	}
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	d := &net.Dialer{Timeout: 20 * time.Second}
	var conn net.Conn
	var err error
	if m.Port == 465 {
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: m.Host})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(60 * time.Second))
	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if ok, _ := c.Extension("STARTTLS"); ok && m.Port != 465 {
		if err := c.StartTLS(&tls.Config{ServerName: m.Host}); err != nil {
			return err
		}
	}
	if m.User != "" {
		if err := c.Auth(smtp.PlainAuth("", m.User, m.Pass, m.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(m.From); err != nil {
		return err
	}
	for _, to := range m.To {
		if err := c.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(m.Message(subject, text, html, time.Now())); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
