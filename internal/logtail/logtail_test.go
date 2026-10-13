package logtail

import (
	"testing"
	"time"
)

func TestParseChat(t *testing.T) {
	c, ok := ParseChat("  3:41 say;1000000000000000001;0;^1Pla^0yer;\x15gg; well played")
	if !ok || c.GUID != "1000000000000000001" || c.NameClean != "Player" || c.Message != "gg; well played" || c.Team {
		t.Fatalf("got %+v %v", c, ok)
	}
	if c, ok := ParseChat("12:00 sayteam;1;2;x;rush B"); !ok || !c.Team || c.Message != "rush B" {
		t.Fatalf("sayteam: %+v", c)
	}
	if g, n, ok := ParseIdentity("  3:22 J;231;0;^1Babou"); !ok || g != "231" || n != "^1Babou" {
		t.Fatalf("join: %q %q %v", g, n, ok)
	}
	if _, _, ok := ParseIdentity("  0:05 J;0;3;bot1"); ok {
		t.Fatal("guid 0 must be ignored")
	}
	if g, _, ok := ParseIdentity("  3:41 say;99;0;x;hi;there"); !ok || g != "99" {
		t.Fatal("say identity")
	}
	for _, l := range []string{"  3:22 J;231;0;Babou", "  4:01 InitGame: \\fs_game\\x", "say;truncated"} {
		if _, ok := ParseChat(l); ok {
			t.Fatalf("%q should not be chat", l)
		}
	}
}

func TestParseEnterLeave(t *testing.T) {
	e, ok := ParseEnterLeave("Tue Oct  6 12:25:55 2026: ^5Client ^1Pla^0yer1^1* 203.0.113.10:64090 ^5entered this server in slot 0 with steamid 76561190000000001 playerid 76561190000000001", time.UTC)
	if !ok || !e.Enter || e.NameRaw != "^1Pla^0yer1^1*" || e.Addr != "203.0.113.10:64090" || e.Slot != 0 || e.SteamID != "76561190000000001" ||
		!e.TS.Equal(time.Date(2026, 10, 6, 12, 25, 55, 0, time.UTC)) {
		t.Fatalf("enter: %+v %v", e, ok)
	}
	e, ok = ParseEnterLeave("Fri Oct 02 16:52:21 2026: ^4Client Unknown Soldier 203.0.113.20:55460 ^4left this server from slot 3 with playerid 1000000000000000001", time.UTC)
	if !ok || e.Enter || e.NameRaw != "Unknown Soldier" || e.Slot != 3 || e.SteamID != "" || e.PlayerID != "1000000000000000001" || e.TS.Day() != 2 {
		t.Fatalf("leave: %+v %v", e, ok)
	}
	if _, ok := ParseEnterLeave("Tue Oct  6 08:09:50 2026: ^4Client bot5 bot ^4left this server from slot 5 with playerid 0", time.UTC); ok {
		t.Fatal("bots must be skipped")
	}
}

func TestParseKill(t *testing.T) {
	k, ok := ParseKill(" 23:28 K;231;0;axis;Babou2312;999;1;allies;[Clan]Player2;m40a3_mp;100;MOD_HEAD_SHOT;head")
	if !ok || k.VictimGUID != "231" || k.VictimName != "Babou2312" || k.KillerGUID != "999" || k.KillerName != "[Clan]Player2" ||
		k.Weapon != "m40a3_mp" || k.Cause != "MOD_HEAD_SHOT" || k.HitLoc != "head" || k.Damage != 100 || k.KillerTeam != "allies" {
		t.Fatalf("kill: %+v %v", k, ok)
	}
	k, ok = ParseKill("  2:10 K;231;0;axis;NoMercy;231;-1;;NoMercy;none;100000;MOD_SUICIDE;none")
	if !ok || k.KillerName != "" || k.Cause != "MOD_SUICIDE" {
		t.Fatalf("suicide must have no killer: %+v", k)
	}
	if _, ok := ParseKill("  2:10 J;231;0;x"); ok {
		t.Fatal("not a kill")
	}
}

func TestParseAccuracy(t *testing.T) {
	a, ok := ParseAccuracy(" 23:38 P_A;1000000000000000002;1;[Clan]Player2;50;14;1400;300;0;0")
	if !ok || a.GUID != "1000000000000000002" || a.Name != "[Clan]Player2" || a.N != [6]int{50, 14, 1400, 300, 0, 0} {
		t.Fatalf("got %+v %v", a, ok)
	}
	if _, ok := ParseAccuracy(" 1:00 P_A;1;2;x;a;0;0;0;0;0"); ok {
		t.Fatal("non numeric must not parse")
	}
}

func TestParseInitGame(t *testing.T) {
	m, gt, ok := ParseInitGame(` 24:10 InitGame: \fs_game\mods/x\g_gametype\sd\mapname\mp_crash\sv_maxclients\64`)
	if !ok || m != "mp_crash" || gt != "sd" {
		t.Fatalf("%q %q %v", m, gt, ok)
	}
}
