package logtail

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var relativeStamp = regexp.MustCompile(`^\s*(\d+):(\d+(?:\.\d+)?)\s+`)

// Anchor is UTC unix nanoseconds at game clock zero. Estimated is true because
// stock games_mp.log contains elapsed time, not an absolute wall-clock timestamp.
type eventClock struct {
	Anchor    int64   `json:"anchor"`
	Last      float64 `json:"last"`
	Estimated bool    `json:"estimated"`
}

func relativeSeconds(line string) (float64, bool) {
	m := relativeStamp.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	mins, e := strconv.ParseFloat(m[1], 64)
	secs, e2 := strconv.ParseFloat(m[2], 64)
	return mins*60 + secs, e == nil && e2 == nil && secs < 60
}
func (c *eventClock) prepare(lines []string, now time.Time) {
	if c.Anchor != 0 {
		return
	}
	// Anchor the first continuous segment at its last observed timestamp.
	var last float64
	found := false
	for _, l := range lines {
		if v, ok := relativeSeconds(l); ok {
			if found && v < last {
				break
			}
			last = v
			found = true
		}
	}
	if found {
		c.Anchor = now.UnixNano() - int64(last*float64(time.Second))
		c.Estimated = true
	}
}
func (c *eventClock) at(line string, now time.Time) time.Time {
	v, ok := relativeSeconds(line)
	if !ok {
		return now
	}
	if c.Anchor == 0 || v < c.Last {
		c.Anchor = now.UnixNano() - int64(v*float64(time.Second))
		c.Estimated = true
	}
	ts := time.Unix(0, c.Anchor+int64(v*float64(time.Second)))
	if ts.After(now.Add(5 * time.Second)) {
		c.Anchor = now.UnixNano() - int64(v*float64(time.Second))
		ts = now
	}
	c.Last = v
	return ts
}

// Unknown engine messages are ignored. Recognised but malformed events are counted.
func malformedGameLine(l string) bool {
	f := fields(l)
	typ := f[0]
	switch typ {
	case "K":
		_, ok := ParseKill(l)
		return !ok
	case "KE":
		_, ok := ParseKillEvent(l)
		return !ok
	case "RE":
		_, ok := ParseRoundEnd(l)
		return !ok
	case "P_A":
		_, ok := ParseAccuracy(l)
		return !ok
	case "MS":
		_, ok := ParseMatchScore(l)
		return !ok
	case "ME":
		_, _, _, ok := ParseMatchEnd(l)
		return !ok
	case "MV", "MVW":
		_, _, _, _, ok := ParseMapVote(l)
		return !ok
	case "PI":
		_, ok := ParseProfile(l)
		return !ok
	case "say", "sayteam":
		_, ok := ParseChat(l)
		return !ok
	case "CL", "P_P", "P_D", "P_HQC", "P_HQD", "P_F":
		_, ok := ParsePlayerEvent(l)
		return !ok
	case "J", "Q":
		return len(f) < 4
	}
	return strings.HasPrefix(typ, "InitGame:") && !strings.Contains(l, `\mapname\`)
}
