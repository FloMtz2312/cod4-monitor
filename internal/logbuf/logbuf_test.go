package logbuf

import (
	"fmt"
	"strings"
	"testing"
)

func TestBuffer(t *testing.T) {
	b := New(3)
	fmt.Fprint(b, "one\ntw")
	fmt.Fprint(b, "o\nthree\n")
	if got := strings.Join(b.Lines(), "|"); got != "one|two|three" {
		t.Fatalf("lines %q", got)
	}
	fmt.Fprint(b, "four\n")
	if got := strings.Join(b.Lines(), "|"); got != "two|three|four" {
		t.Fatalf("ring %q", got)
	}
}

func TestRedact(t *testing.T) {
	for in, want := range map[string]string{
		`{"dir":"ftp://user:s3cret@1.2.3.4:21/mods/x/"}`: `{"dir":"ftp://user:xxxxx@1.2.3.4:21/mods/x/"}`,
		`{"msg":"x","rcon_pass":"abc","n":1}`:            `{"msg":"x","rcon_pass":"xxxxx","n":1}`,
		`{"api_key":"k\"ey","token":"t"}`:                `{"api_key":"xxxxx","token":"xxxxx"}`,
		`set rcon_password hunter2`:                      `[secret masqué]`,
		`{"msg":"log tail connected","server_id":1}`:     `{"msg":"log tail connected","server_id":1}`,
	} {
		if got := Redact(in); got != want {
			t.Errorf("Redact(%s) = %s, want %s", in, got, want)
		}
	}
}
