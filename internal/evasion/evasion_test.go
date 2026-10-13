package evasion

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"cod4monitor/internal/store"
)

func TestScore(t *testing.T) {
	banned := &player{key: "B", clean: "Kevin", ips: map[string]bool{"1.2.3.4": true}, steam: map[string]bool{"765": true},
		level: 45, prestige: 2, k: 12000, settings: "30000/30/100", kills: 100, hs: 50, ns: 20, dist: sql.NullFloat64{Float64: 40, Valid: true}}
	at := int64(1000)
	for _, tc := range []struct {
		name  string
		p     player
		codes string
		score int
	}{
		{"same IP, new GUID", player{ips: map[string]bool{"1.2.3.4": true}, first: 2000}, "ip new", 70},
		{"same Steam id", player{steam: map[string]bool{"765": true}, first: 500}, "steam", 60},
		{"spoofer behind a VPN: same profile and name", player{clean: "K3vin", level: 47, prestige: 2, k: 12400, first: 2000}, "profile name new", 75},
		{"same subnet, veteran, same settings and style", player{ips: map[string]bool{"1.2.3.9": true}, first: 2000, level: 30, settings: "30000/30/100",
			kills: 50, hs: 26, ns: 9, dist: sql.NullFloat64{Float64: 43, Valid: true}}, "subnet settings style veteran new", 75},
		{"only a veteran newcomer: not linked to this ban", player{first: 2000, level: 50, prestige: 3, k: 30000}, "", 0},
		{"default settings don't count", player{settings: "25000/20/30"}, "", 0},
		{"close name, new GUID", player{clean: "Kevinn", first: 2000}, "name_near new", 25},
		{"known before the ban, only weak clues", player{clean: "Kevinn", ips: map[string]bool{"1.2.3.9": true}, first: 500}, "", 0},
	} {
		s := Score(&tc.p, banned, at)
		codes := ""
		for _, r := range s.Reasons {
			if codes != "" {
				codes += " "
			}
			codes += r.Code
		}
		if codes != tc.codes || s.Score != tc.score {
			t.Errorf("%s: %q %d, want %q %d", tc.name, codes, s.Score, tc.codes, tc.score)
		}
	}
}

func TestFind(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	conn := func(guid, name, ip, steam string, at time.Time) {
		st.DB.Exec(`INSERT INTO connections(server_id, slot, addr, ip, steamid, guid, name_raw, name_clean, joined_at, left_at, open)
			VALUES(1,0,?,?,?,?,?,?,?,?,0)`, ip+":1", ip, steam, guid, name, name, at.Unix(), at.Unix()+600)
	}
	conn("G1", "Kevin", "10.0.0.1", "765", now.Add(-72*time.Hour))
	st.AddProfile(1, now.Add(-72*time.Hour), "G1", "Kevin", "Kevin", "765", 45, 2, 12000, 9000, 5000, 300)
	st.AddBan(1, now.Add(-48*time.Hour), "G1", "Kevin", "10.0.0.1", "", "wallhack", "permban")
	// back with a spoofed GUID, through a VPN, same Steam id and career
	conn("G9", "xX_Kev_Xx", "99.1.1.1", "765", now.Add(-time.Hour))
	st.AddProfile(1, now.Add(-time.Hour), "G9", "xX_Kev_Xx", "xX_Kev_Xx", "765", 46, 2, 12150, 9100, 5050, 301)
	// a regular player with nothing in common
	conn("G5", "Omni", "20.0.0.5", "", now.Add(-time.Hour))
	st.AddProfile(1, now.Add(-time.Hour), "G5", "Omni", "Omni", "", 50, 4, 40000, 20000, 9000, 900)

	sus, err := Find(st.DB, 1, now.Add(-30*24*time.Hour).Unix())
	if err != nil {
		t.Fatal(err)
	}
	if len(sus) != 1 || sus[0].Key != "G9" || sus[0].BanKey != "G1" || sus[0].Score != ptSteam+ptProfile+ptNew {
		t.Fatalf("%+v", sus)
	}
}
