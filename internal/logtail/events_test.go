package logtail

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestModLines(t *testing.T) {
	ke, ok := ParseKillEvent(`  12:34 KE;1234abcd;3;^1Babou;5678ef;7;Omni;523;hs,ns;225;-1`)
	if !ok || ke.VictimSlot != 7 || ke.Distance != 52.3 || ke.Tags != "hs,ns" || ke.Points != 225 || ke.AdsMs != -1 || ke.KillerName != "^1Babou" {
		t.Fatalf("KE: %+v %v", ke, ok)
	}
	if re, ok := ParseRoundEnd(`RE;12;allies;7;5;axis`); !ok || re.Winner != "allies" || re.Allies != 7 || re.Axis != 5 || re.Attackers != "axis" {
		t.Fatalf("RE: %+v", re)
	}
	if e, ok := ParsePlayerEvent(`CL;abc;2;Omni;3`); !ok || e.Type != "clutch" || e.Value != 3 || e.Name != "Omni" {
		t.Fatalf("CL: %+v", e)
	}
	if e, ok := ParsePlayerEvent(`P_P;abc;2;Omni`); !ok || e.Type != "plant" {
		t.Fatalf("P_P: %+v", e)
	}
	if m, ok := ParseMatchScore(`MS;abc;2;Omni;axis;1450;12;9;23;1`); !ok || m.N != [5]int{1450, 12, 9, 23, 1} || m.Team != "axis" {
		t.Fatalf("MS: %+v", m)
	}
	if w, a, x, ok := ParseMatchEnd(`ME;axis;11;13`); !ok || w != "axis" || a != 11 || x != 13 {
		t.Fatal("ME")
	}
	if mp, mode, v, chosen, ok := ParseMapVote(`MV;mp_crash;sd;4`); !ok || mp != "mp_crash" || mode != "sd" || v != 4 || chosen {
		t.Fatal("MV")
	}
	if mp, _, _, chosen, ok := ParseMapVote(`MVW;mp_crash;dm`); !ok || mp != "mp_crash" || !chosen {
		t.Fatal("MVW")
	}
	if g, slot, name, ok := ParseJoin(`J;abc;4;^2Om;ni`); !ok || g != "abc" || slot != 4 || name != "^2Om;ni" {
		t.Fatalf("J: %s %d %s", g, slot, name)
	}
	if p, ok := ParseProfile(`PI;abc;4;^2Omni;76561198;45;2;12000;9000;5000;300`); !ok || p.PlayerID != "76561198" || p.N != [6]int{45, 2, 12000, 9000, 5000, 300} {
		t.Fatalf("PI: %+v", p)
	}
	info := ParseUserinfo("userinfo\n--------\nname            ^2Omni\nrate            30000\nsnaps           30\ncl_maxpackets   100\n")
	if info["rate"] != "30000" || info["cl_maxpackets"] != "100" || info["name"] != "^2Omni" || info["--------"] != "" {
		t.Fatalf("dumpuser: %v", info)
	}
	if c, ok := ParseChat("say;abc;5;Omni;\x15!stats"); !ok || c.Slot != 5 || c.Message != "!stats" {
		t.Fatalf("chat: %+v", c)
	}
}

// A player name or argument must never end the rcon command (";" runs another command, quotes close the message).
func TestSafeTell(t *testing.T) {
	if s := safe(`x"; rcon_password hacked; say "`); s != `x rcon_password hacked say ` {
		t.Fatalf("%q", s)
	}
}

func TestGameEventsStored(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	tl := &Tail{ServerID: 1}
	now := time.Now()
	emit := func(map[string]any) {}
	st.StartRound(1, now, "mp_crash", "sd")
	lines := []string{
		`KE;111;3;A;222;7;B;400;hs,ns;225;-1`,
		`K;222;7;allies;B;111;3;axis;A;m40a3_mp;135;MOD_HEAD_SHOT;head`,
		`K;333;8;allies;C;111;3;axis;A;m40a3_mp;135;MOD_RIFLE_BULLET;torso_upper`, // no KE line: no details
		`P_P;111;3;A`,
		`RE;1;axis;0;1;axis`,
		`CL;111;3;A;2`,
		`MS;111;3;A;axis;450;2;0;12;0`,
		`ME;axis;0;1`,
		`MV;mp_crash;sd;2`, `MV;mp_bog;sd;1`, `MVW;mp_crash;sd`,
	}
	for _, l := range lines {
		if !tl.gameEvent(st, l, now, true, emit) {
			if k, ok := ParseKill(l); ok {
				st.AddKill(1, now, "111", k.KillerName, k.KillerName, "axis", k.VictimGUID, k.VictimName, k.VictimName, "allies", k.Weapon, k.Cause, k.HitLoc, k.Damage)
				if ke := tl.pending; ke != nil && ke.VictimSlot == k.VictimSlot {
					st.SetLastKillExtra(1, store.KillExtra{Distance: ke.Distance, Tags: ke.Tags, Points: ke.Points, AdsMs: ke.AdsMs})
				}
				tl.pending = nil
			}
		}
	}
	var dist float64
	var tags string
	var n int
	st.DB.QueryRow(`SELECT distance, tags FROM kills WHERE victim_key='222'`).Scan(&dist, &tags)
	st.DB.QueryRow(`SELECT COUNT(*) FROM kills WHERE distance IS NULL`).Scan(&n)
	if dist != 40 || tags != "hs,ns" || n != 1 {
		t.Fatalf("kill details: %v %q, without: %d", dist, tags, n)
	}
	var winner string
	st.DB.QueryRow(`SELECT winner FROM rounds`).Scan(&winner)
	var events, scores, chosen int
	st.DB.QueryRow(`SELECT COUNT(*) FROM game_events WHERE round_id IS NOT NULL`).Scan(&events)
	st.DB.QueryRow(`SELECT COUNT(*) FROM match_scores WHERE key='111' AND level=12`).Scan(&scores)
	st.DB.QueryRow(`SELECT COUNT(*) FROM map_votes WHERE chosen=1 AND map='mp_crash'`).Scan(&chosen)
	if winner != "axis" || events != 2 || scores != 1 || chosen != 1 {
		t.Fatalf("winner %q events %d scores %d chosen %d", winner, events, scores, chosen)
	}
}

func TestChatCommands(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	var sent []string
	tl := &Tail{ServerID: 1, Rcon: func(cmd string) (string, error) { sent = append(sent, cmd); return "", nil }}
	now := time.Now()
	st.AddKill(1, now, "111", "^1Babou", "Babou", "axis", "222", "Omni", "Omni", "allies", "m40a3_mp", "MOD_HEAD_SHOT", "head", 135)
	st.SetLastKillExtra(1, store.KillExtra{Distance: 61.4, Tags: "hs,ns", Points: 225, AdsMs: -1})
	var events []map[string]any
	emit := func(e map[string]any) { events = append(events, e) }

	tl.command(st, Chat{GUID: "111", NameRaw: "^1Babou", NameClean: "Babou", Message: "!stats", Slot: 3}, emit)
	if len(sent) != 1 || sent[0] != `tell 3 "^3Babou^7: 1 kills, 0 deaths, K/D 1.00, HS 100%, 1 noscopes, longest 61m"` {
		t.Fatalf("stats: %q", sent)
	}
	tl.command(st, Chat{GUID: "111", NameRaw: "^1Babou", NameClean: "Babou", Message: "!top", Slot: 3}, emit)
	if len(sent) != 1 {
		t.Fatalf("cooldown not applied: %q", sent)
	}
	tl.lastCmd = nil
	tl.command(st, Chat{GUID: "222", NameRaw: "Omni", NameClean: "Omni", Message: `!report Cheat"er; quit`, Slot: 5}, emit)
	var target string
	st.DB.QueryRow(`SELECT target FROM reports`).Scan(&target)
	if target != `Cheat"er;` || len(events) != 1 || events[0]["type"] != "report" || sent[1] != `tell 5 "^2Report sent to the admins, thanks."` {
		t.Fatalf("report: %q %v %q", target, events, sent)
	}
	tl.lastCmd = nil
	tl.command(st, Chat{GUID: "222", NameRaw: "Omni", NameClean: "Omni", Message: `!stats Bab";quit`, Slot: 5}, emit)
	if last := sent[len(sent)-1]; strings.Count(last, `"`) != 2 || strings.Contains(last, ";") {
		t.Fatalf("injection: %q", last)
	}
	tl.lastCmd = nil
	tl.SiteURL = "https://cod4.example.org/"
	tl.command(st, Chat{GUID: "111", NameRaw: "^1Babou", NameClean: "Babou", Message: "!me", Slot: 3}, emit)
	var code string
	st.DB.QueryRow(`SELECT code FROM me_codes WHERE key='111'`).Scan(&code)
	if want := `tell 3 "^7Your stats: ^3cod4.example.org/me/` + code + ` ^7(valid 24 h)"`; code == "" || sent[len(sent)-1] != want {
		t.Fatalf("me: %q, want %q", sent[len(sent)-1], want)
	}
	if tl.command(st, Chat{Message: "!unknown"}, emit) {
		t.Fatal("unknown command handled")
	}
}
