// Package logbuf keeps the last lines the monitor logged, for the super admin's page: the same lines as
// "docker logs", without giving the monitor access to Docker. Passwords and keys are masked; nothing is written to disk.
package logbuf

import (
	"regexp"
	"strings"
	"sync"

	"cod4monitor/internal/query"
)

// Buffer is an io.Writer keeping the last lines written to it.
type Buffer struct {
	mu      sync.Mutex
	lines   []string
	next    int
	full    bool
	partial string
}

// New keeps up to n lines.
func New(n int) *Buffer { return &Buffer{lines: make([]string, n)} }

var (
	userinfo = regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://[^:/@\s"]+):[^@\s"]+@`)
	secretKV = regexp.MustCompile(`(?i)("[a-z_]*(?:pass|password|passwd|token|secret|key|auth|cookie)[a-z_]*"\s*:\s*)"(?:[^"\\]|\\.)*"`)
)

// Redact masks what must never be shown: passwords in addresses, values of secret-looking fields, rcon passwords.
func Redact(line string) string {
	line = userinfo.ReplaceAllString(line, "${1}:xxxxx@")
	line = secretKV.ReplaceAllString(line, `${1}"xxxxx"`)
	return query.RedactSecrets(line)
}

func (b *Buffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	parts := strings.Split(b.partial+string(p), "\n")
	b.partial = parts[len(parts)-1]
	if len(b.partial) > 64*1024 {
		parts = append(parts[:len(parts)-1], b.partial, "")
		b.partial = ""
	}
	for _, l := range parts[:len(parts)-1] {
		if l = strings.TrimRight(l, "\r"); l == "" {
			continue
		}
		if len(l) > 8*1024 {
			l = l[:8*1024] + "…"
		}
		b.lines[b.next] = Redact(l)
		b.next = (b.next + 1) % len(b.lines)
		b.full = b.full || b.next == 0
	}
	return len(p), nil
}

// Lines returns the kept lines, oldest first.
func (b *Buffer) Lines() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.full {
		return append([]string{}, b.lines[:b.next]...)
	}
	return append(append([]string{}, b.lines[b.next:]...), b.lines[:b.next]...)
}
