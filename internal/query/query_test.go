package query

import (
	"testing"
	"time"
)

func TestParseWithPlayers(t *testing.T) {
	raw := "\xff\xff\xff\xffstatusResponse\n" +
		`\sv_hostname\^1UpT# ^7Sniper\mapname\mp_crash\g_gametype\sd\sv_maxclients\18` + "\n" +
		`5 48 "^1Low"` + "\n" +
		`12 70 "^2Top^7Guy"` + "\n"
	st, err := Parse([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if st.Map != "mp_crash" || st.Gametype != "sd" || st.MaxPlayers != 18 || st.Hostname != "^1UpT# ^7Sniper" {
		t.Fatalf("bad vars: %+v", st)
	}
	if len(st.Players) != 2 {
		t.Fatalf("want 2 players, got %d", len(st.Players))
	}
	p := st.Players[0] // sorted by score desc
	if p.NameClean != "TopGuy" || p.NameRaw != "^2Top^7Guy" || p.Score != 12 || p.Ping != 70 {
		t.Fatalf("bad player: %+v", p)
	}
}

func TestParseEmptyServer(t *testing.T) {
	st, err := Parse([]byte("\xff\xff\xff\xffstatusResponse\n\\mapname\\mp_nuketown\\sv_maxclients\\18\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Players) != 0 || st.Map != "mp_nuketown" {
		t.Fatalf("got %+v", st)
	}
}

func TestParseTruncated(t *testing.T) {
	// Last player line cut mid-name: must be skipped, not crash.
	st, err := Parse([]byte("\xff\xff\xff\xffstatusResponse\n\\mapname\\mp_x\n3 50 \"ok\"\n7 40 \"cut"))
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Players) != 1 || st.Players[0].NameClean != "ok" {
		t.Fatalf("got %+v", st.Players)
	}
	if _, err := Parse([]byte("\xff\xff\xff\xffstatusResp")); err == nil {
		t.Fatal("want error on truncated header")
	}
	if _, err := Parse([]byte("\xff\xff\xff\xffstatusResponse")); err == nil {
		t.Fatal("want error on missing info line")
	}
}

func TestRconPayload(t *testing.T) {
	if got := rconPayload([]byte("\xff\xff\xff\xffprint\nmap: mp_crash\n")); got != "map: mp_crash\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := Rcon("127.0.0.1:1", "x", "say a\nquit", time.Second); err == nil {
		t.Fatal("multi-line command must be refused")
	}
}

func TestParseRconStatus(t *testing.T) {
	out := "map: mp_backlot\n" +
		"num score ping guid                             name            lastmsg address               qport rate\n" +
		"--- ----- ---- -------------------------------- --------------- ------- --------------------- ----- -----\n" +
		"  0    12   48 1000000000000000001 ^1Pla^0yer1^1*^7       0 203.0.113.10:51918    12345 25000\n" +
		"  3     0  999 0 Unknown Soldier^7      50 203.0.113.20:55460   4321 5000\n" +
		"  5     0 CNCT 0 bot5^7       0 bot   0 0\n"
	ps := ParseRconStatus(out)
	if len(ps) != 3 {
		t.Fatalf("got %+v", ps)
	}
	if p := ps[0]; p.Num != 0 || p.Name != "^1Pla^0yer1^1*" || p.GUID != "1000000000000000001" || p.Addr != "203.0.113.10:51918" {
		t.Fatalf("p0 %+v", p)
	}
	if p := ps[1]; p.Num != 3 || p.Name != "Unknown Soldier" {
		t.Fatalf("p1 %+v", p)
	}
}

// CoD4X 1.8 layout (sv_cmds.c SV_Status_f): playerid padded to 20, steamid to 18, name + ^7 padded to 33, lastmsg %7i.
func TestParseRconStatusCoD4X(t *testing.T) {
	out := "hostname: ^1UpT^0#^7 Sniper\nversion : CoD4 X 1.8\nudp/ip  : 0.0.0.0:28960\nos      : win\ntype    : dedicated server\nmap     : mp_crash\n\n" +
		"num score ping playerid            steamid           name                             lastmsg address                                              qport rate\n" +
		"--- ----- ---- ------------------- ----------------- -------------------------------- ------- ---------------------------------------------------- ----- -----\n" +
		"  0    40   35 2310346614893848611 76561198000000001 ^1Ba^7bou^7                              0 203.0.113.10:28960                                   12345 25000\n" +
		"  2     0 PRIM 2310346614893848612 0                 Unknown Soldier^7                      50 203.0.113.20:55460                                    4321  5000\n"
	ps := ParseRconStatus(out)
	if len(ps) != 2 {
		t.Fatalf("got %+v", ps)
	}
	if p := ps[0]; p.Num != 0 || p.Name != "^1Ba^7bou" || p.GUID != "2310346614893848611" || p.Addr != "203.0.113.10:28960" {
		t.Fatalf("p0 %+v", p)
	}
	if p := ps[1]; p.Num != 2 || p.Name != "Unknown Soldier" || p.GUID != "2310346614893848612" {
		t.Fatalf("p1 %+v", p)
	}
}

func TestLocalizeDates(t *testing.T) {
	paris, err := time.LoadLocation("Europe/Paris")
	if err != nil {
		t.Skip("no tzdata")
	}
	got := LocalizeDates("Logfile opened on Tue Oct  6 13:11:11 2026 / Fri Oct 02 16:52:21 2026", time.UTC, paris)
	if want := "Logfile opened on 06/10/2026 15:11:11 / 02/10/2026 18:52:21"; got != want {
		t.Fatalf("got %q", got)
	}
	if got := LocalizeDates("Wed Jan 14 10:00:00 2026", time.UTC, paris); got != "14/01/2026 11:00:00" { // winter: UTC+1
		t.Fatalf("winter: %q", got)
	}
}

func TestHostOf(t *testing.T) {
	for in, want := range map[string]string{"1.2.3.4:28960": "1.2.3.4", "[2a01:e0a::1]:28960": "2a01:e0a::1", "1.2.3.4": "1.2.3.4", "loopback": "loopback"} {
		if got := HostOf(in); got != want {
			t.Errorf("HostOf(%q) = %q, want %q", in, got, want)
		}
	}
}
