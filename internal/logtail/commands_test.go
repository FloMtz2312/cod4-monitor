package logtail

import (
	"testing"

	"cod4monitor/internal/store"
)

func TestHelpLeftToB3(t *testing.T) {
	for _, b3db := range []string{"", "/b3/b3.db"} {
		var sent []string
		tl := &Tail{Files: store.LogFiles{B3DB: b3db}, Rcon: func(cmd string) (string, error) { sent = append(sent, cmd); return "", nil }}
		handled := tl.command(nil, Chat{GUID: "g1", Slot: 3, Message: "!help"}, func(map[string]any) {})
		if want := b3db == ""; handled != want || (len(sent) == 1) != want {
			t.Errorf("B3DB %q: handled %v, rcon %q", b3db, handled, sent)
		}
	}
}
