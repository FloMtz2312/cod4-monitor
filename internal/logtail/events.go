package logtail

import (
	"strconv"
	"strings"
)

// Lines written by the pm_sniper mod for the dashboard (promod/gamelog.gsc), plus the stock objective lines.

// KillEvent is a KE line: KE;aGUID;aSlot;aName;vGUID;vSlot;vName;distance_dm;tags;points;adsMs (right before its K line).
type KillEvent struct {
	VictimSlot    int
	Distance      float64 // meters
	Tags          string
	Points, AdsMs int
	KillerName    string
}

func fields(line string) []string { return strings.Split(stampRe.ReplaceAllString(line, ""), ";") }

func ParseKillEvent(line string) (KillEvent, bool) {
	f := fields(line)
	if len(f) != 11 || f[0] != "KE" {
		return KillEvent{}, false
	}
	slot, e1 := strconv.Atoi(f[5])
	dm, e2 := strconv.Atoi(f[7])
	pts, e3 := strconv.Atoi(f[9])
	ads, e4 := strconv.Atoi(f[10])
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		return KillEvent{}, false
	}
	return KillEvent{VictimSlot: slot, Distance: float64(dm) / 10, Tags: f[8], Points: pts, AdsMs: ads, KillerName: f[3]}, true
}

// RoundEnd is a RE line: RE;round;winner;alliesScore;axisScore;attackers
type RoundEnd struct {
	Winner, Attackers string
	Allies, Axis      int
}

func ParseRoundEnd(line string) (RoundEnd, bool) {
	f := fields(line)
	if len(f) != 6 || f[0] != "RE" {
		return RoundEnd{}, false
	}
	a, e1 := strconv.Atoi(f[3])
	x, e2 := strconv.Atoi(f[4])
	if e1 != nil || e2 != nil {
		return RoundEnd{}, false
	}
	return RoundEnd{Winner: f[2], Allies: a, Axis: x, Attackers: f[5]}, true
}

// PlayerEvent: a player did something (event type, guid, name, value).
type PlayerEvent struct {
	Type, GUID, Name string
	Value            int
}

// objective lines of the stock gametypes: P_P;guid;slot;name (bomb planted) ...
var objectives = map[string]string{"P_P": "plant", "P_D": "defuse", "P_HQC": "hq_capture", "P_HQD": "hq_destroy", "P_F": "flag"}

// ParsePlayerEvent reads the objective lines and CL;guid;slot;name;vs (clutch won against vs enemies).
func ParsePlayerEvent(line string) (PlayerEvent, bool) {
	f := fields(line)
	if typ, ok := objectives[f[0]]; ok && len(f) == 4 {
		return PlayerEvent{Type: typ, GUID: f[1], Name: f[3]}, true
	}
	if f[0] == "CL" && len(f) == 5 {
		vs, err := strconv.Atoi(f[4])
		return PlayerEvent{Type: "clutch", GUID: f[1], Name: f[3], Value: vs}, err == nil
	}
	return PlayerEvent{}, false
}

// MatchScore is a MS line: MS;guid;slot;name;team;score;kills;deaths;level;prestige
type MatchScore struct {
	GUID, Name, Team string
	N                [5]int // score, kills, deaths, level, prestige
}

func ParseMatchScore(line string) (MatchScore, bool) {
	f := fields(line)
	if len(f) != 10 || f[0] != "MS" {
		return MatchScore{}, false
	}
	m := MatchScore{GUID: f[1], Name: f[3], Team: f[4]}
	for i := range m.N {
		var err error
		if m.N[i], err = strconv.Atoi(f[5+i]); err != nil {
			return MatchScore{}, false
		}
	}
	return m, true
}

// ParseMatchEnd reads ME;winner;alliesScore;axisScore.
func ParseMatchEnd(line string) (winner string, allies, axis int, ok bool) {
	f := fields(line)
	if len(f) != 4 || f[0] != "ME" {
		return "", 0, 0, false
	}
	a, e1 := strconv.Atoi(f[2])
	x, e2 := strconv.Atoi(f[3])
	return f[1], a, x, e1 == nil && e2 == nil
}

// ParseMapVote reads MV;map;mode;votes (a card of the vote) and MVW;map;mode (the chosen map, chosen=true).
func ParseMapVote(line string) (mapName, mode string, votes int, chosen, ok bool) {
	f := fields(line)
	switch {
	case f[0] == "MV" && len(f) == 4:
		v, err := strconv.Atoi(f[3])
		return f[1], f[2], v, false, err == nil
	case f[0] == "MVW" && len(f) == 3:
		return f[1], f[2], 0, true, true
	}
	return "", "", 0, false, false
}

// Profile is a PI line: PI;guid;slot;name;playerid;level;prestige;careerKills;careerDeaths;careerHeadshots;careerMatches
type Profile struct {
	GUID, Name, PlayerID string
	N                    [6]int // level, prestige, kills, deaths, headshots, matches
}

func ParseProfile(line string) (Profile, bool) {
	f := fields(line)
	if len(f) != 11 || f[0] != "PI" {
		return Profile{}, false
	}
	p := Profile{GUID: f[1], Name: f[3], PlayerID: f[4]}
	for i := range p.N {
		var err error
		if p.N[i], err = strconv.Atoi(f[5+i]); err != nil {
			return Profile{}, false
		}
	}
	return p, true
}

// ParseUserinfo reads the output of rcon "dumpuser": a "key value" line per userinfo key.
func ParseUserinfo(out string) map[string]string {
	info := map[string]string{}
	for _, l := range strings.Split(out, "\n") {
		if k, v, ok := strings.Cut(strings.TrimSpace(l), " "); ok && k != "" && !strings.HasPrefix(k, "-") {
			info[k] = strings.TrimSpace(v)
		}
	}
	return info
}

// ParseJoin reads J;guid;slot;name (a player connects).
func ParseJoin(line string) (guid string, slot int, name string, ok bool) {
	f := strings.SplitN(stampRe.ReplaceAllString(line, ""), ";", 4)
	if len(f) != 4 || f[0] != "J" {
		return "", 0, "", false
	}
	slot, err := strconv.Atoi(f[2])
	return f[1], slot, f[3], err == nil
}
