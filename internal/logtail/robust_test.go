package logtail

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

// A line too long to keep (NUL bytes left by a crash) is dropped up to its end; the file keeps being read.
func TestOversizedLineDoesNotBlock(t *testing.T) {
	tl, st, path := recoveryFixture(t)
	junk := bytes.Repeat([]byte{0}, 3*maxRead+100) // spans several reads
	os.WriteFile(path, append(append([]byte("before\n"), junk...), []byte("end of junk\nafter one\nafter two\n")...), 0600)
	f := chatFile(t)
	for i := 0; i < 10; i++ {
		if e := tl.poll(localSource{}, st, f); e != nil {
			t.Fatalf("poll %d: %v", i, e)
		}
	}
	var msgs []string
	rs, _ := st.DB.Query(`SELECT message FROM chat_messages ORDER BY id`)
	for rs.Next() {
		var m string
		rs.Scan(&m)
		msgs = append(msgs, m)
	}
	rs.Close()
	if got := strings.Join(msgs, "|"); got != "before|after one|after two" {
		t.Fatalf("lines %q", got)
	}
}

// A file in error doesn't stop the others; a renewed FTP session isn't an error.
func TestFileErrorDoesNotStopOthers(t *testing.T) {
	tl, st, path := recoveryFixture(t)
	os.WriteFile(path, []byte("hello\n"), 0600)
	tl.Every = 10 * time.Millisecond
	broken := &file{name: "/outside/the/root.log", persist: true, fromStart: true, handle: func(*store.Store, []string, bool) {}}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := tl.follow(ctx, st, []*file{broken, chatFile(t)})
	if err != nil {
		t.Fatalf("follow: %v", err)
	}
	if n := countChats(t, st); n != 1 {
		t.Fatalf("chat lines: %d", n)
	}
}
