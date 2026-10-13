package mail

import (
	"bufio"
	"encoding/base64"
	"net"
	"strconv"
	"strings"
	"testing"
)

// fakeSMTP answers one session (no TLS, no auth) and returns the DATA it received.
func fakeSMTP(t *testing.T) (port int, got chan string) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	got = make(chan string, 1)
	go func() {
		defer ln.Close()
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r, w := bufio.NewReader(c), c
		w.Write([]byte("220 fake\r\n"))
		var data strings.Builder
		inData := false
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return
			}
			if inData {
				if l == ".\r\n" {
					inData = false
					w.Write([]byte("250 ok\r\n"))
					got <- data.String()
					continue
				}
				data.WriteString(l)
				continue
			}
			switch cmd := strings.ToUpper(strings.Fields(l)[0]); cmd {
			case "EHLO":
				w.Write([]byte("250-fake\r\n250 8BITMIME\r\n"))
			case "DATA":
				inData = true
				w.Write([]byte("354 go\r\n"))
			case "QUIT":
				w.Write([]byte("221 bye\r\n"))
				return
			default:
				w.Write([]byte("250 ok\r\n"))
			}
		}
	}()
	return ln.Addr().(*net.TCPAddr).Port, got
}

func TestSend(t *testing.T) {
	port, got := fakeSMTP(t)
	t.Setenv("SMTP_HOST", "127.0.0.1")
	t.Setenv("SMTP_PORT", strconv.Itoa(port))
	t.Setenv("MAIL_FROM", "monitor@example.com")
	t.Setenv("MAIL_TO", "a@example.com, b@example.com")
	m := FromEnv()
	if m == nil || len(m.To) != 2 {
		t.Fatalf("FromEnv: %+v", m)
	}
	if err := m.Send("Résumé de la semaine", "texte é", "<b>html</b>"); err != nil {
		t.Fatal(err)
	}
	msg := <-got
	if !strings.Contains(msg, "Subject: =?utf-8?q?R=C3=A9sum=C3=A9_de_la_semaine?=") || !strings.Contains(msg, "multipart/alternative") {
		t.Fatalf("headers: %s", msg)
	}
	if !strings.Contains(msg, base64.StdEncoding.EncodeToString([]byte("texte é"))) {
		t.Fatalf("body: %s", msg)
	}
	var nilMailer *Mailer
	if err := nilMailer.Send("x", "y", ""); err != nil {
		t.Fatal(err)
	}
}

func TestFromEnvDisabled(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("MAIL_TO", "a@example.com")
	if FromEnv() != nil {
		t.Fatal("want nil without SMTP_HOST")
	}
}
