package poller

import (
	"testing"
	"time"
)

func TestRestarted(t *testing.T) {
	for _, c := range []struct {
		prev, cur string
		elapsed   time.Duration
		want      bool
	}{
		{"12 minutes", "12 minutes", 15 * time.Second, false},
		{"12 minutes", "0 minutes", 15 * time.Second, true},
		{"239 minutes", "4 hours", 15 * time.Second, false}, // unit change
		{"5 hours", "5 hours", 15 * time.Second, false},     // hours only change once an hour
		{"71 hours", "3 days", 15 * time.Second, false},
		{"5 hours", "2 minutes", 15 * time.Second, true},
		{"3 minutes", "1 minutes", 10 * time.Minute, true}, // down 10 min, back with a new process
		{"3 minutes", "13 minutes", 10 * time.Minute, false},
		{"", "1 minutes", 15 * time.Second, false},
		{"12 minutes", "", 15 * time.Second, false},
	} {
		if got := Restarted(c.prev, c.cur, c.elapsed); got != c.want {
			t.Errorf("Restarted(%q, %q, %s) = %v", c.prev, c.cur, c.elapsed, got)
		}
	}
}
