package api

import "testing"

func TestInferRounds(t *testing.T) {
	// S&D 2 v 2 logged as one database round (no RE line): allies A1, A2 against axis X1, X2
	var kills []roundKill
	ts := int64(1000)
	kill := func(killer, kteam, victim, vteam string) {
		ts += 5
		kills = append(kills, roundKill{ID: int64(len(kills) + 1), Round: 7, TS: ts, Killer: killer, KillerName: "^1" + killer, KillerTeam: kteam,
			Victim: victim, VictimTeam: vteam})
	}
	kill("x1", "axis", "a1", "allies") // round 1: A1 dies first (the other allies are not known yet), then the allies win
	kill("a2", "allies", "x1", "axis")
	kill("a2", "allies", "x2", "axis")
	ts += 30
	kill("x1", "axis", "a2", "allies") // round 2: A2 was alive at the end of round 1; the axis win
	kill("x2", "axis", "a1", "allies")
	ts += 30
	kill("a1", "allies", "x1", "axis") // round 3: A2 plants, the axis kill everybody, nobody defuses: the bomb wins
	plant := ts + 2
	kill("x2", "axis", "a2", "allies")
	kill("x2", "axis", "a1", "allies")
	ts += 30
	kill("a1", "allies", "x2", "axis") // round 4: no winner known (only one death)
	events := []roundEvent{{Round: 7, TS: plant, Type: "plant", Key: "a2", Text: "plant:A2:0"}}
	rs := inferRounds("sd", []playedRound{{ID: 7, Start: 990}}, kills, events)
	if len(rs) != 4 {
		t.Fatalf("%d rounds: %+v", len(rs), rs)
	}
	want := []string{"allies", "axis", "allies", ""}
	for i, r := range rs {
		if r.Winner != want[i] {
			t.Errorf("round %d: winner %q, want %q", i+1, r.Winner, want[i])
		}
	}
	if rs[0].ID != 7 || rs[1].ID >= 0 || rs[0].Kills != 3 || rs[0].FirstBlood != "^1x1" || rs[0].TopKiller != "^1a2" || rs[2].Events != "plant:A2:0" {
		t.Fatalf("details: %+v", rs)
	}
	if *rs[3].ScoreAllies != 2 || *rs[3].ScoreAxis != 1 {
		t.Fatalf("score %d-%d", *rs[3].ScoreAllies, *rs[3].ScoreAxis)
	}
	if w, a, x, ok := estimatedResult("sd", rs, kills); !ok || w != "allies" || a != 2 || x != 1 {
		t.Fatalf("result %q %d-%d %v", w, a, x, ok)
	}

	// a mod that writes its results (RE): its rounds stay as they are
	official := inferRounds("sd", []playedRound{{ID: 7, Start: 990, Winner: "axis"}}, kills, nil)
	if len(official) != 1 || official[0].Winner != "axis" || official[0].Estimated {
		t.Fatalf("official: %+v", official)
	}

	// team deathmatch: one round, the team with the most kills wins the match
	tdm := inferRounds("war", []playedRound{{ID: 7, Start: 990}}, kills, nil)
	if len(tdm) != 1 || tdm[0].Winner != "" {
		t.Fatalf("war: %+v", tdm)
	}
	if w, a, x, ok := estimatedResult("war", tdm, kills); !ok || w != "axis" || a != 4 || x != 5 {
		t.Fatalf("war result %q %d-%d %v", w, a, x, ok)
	}
	if _, _, _, ok := estimatedResult("dom", tdm, kills); ok {
		t.Fatal("domination: kills don't decide")
	}
}
