package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLacksIslandwalkWord: !hasKeywordLandwalk:Island lowers to PredWord7 /
// W7LacksIslandwalk (no payload; the kernel reads the Island subtype off the matchup), the
// Island landwalk spec lowers to a clause holding only that Sub (the shape the
// kernel's landwalkNames reads), and the other spellings stay refused.
func TestLacksIslandwalkWord(t *testing.T) {
	cb := &cardBuilder{pt: &poolTables{subIdx: map[string]int{"Island": 6}, missing: map[string]bool{}}}
	f, err := parseFilter("Creature.withoutFlying+!hasKeywordLandwalk:Island",
		func(w string) bool { return w == "Island" }, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
	if err != nil {
		t.Fatal(err)
	}
	if kf.Pred != kop.PredWord7 || kf.PredN != kop.W7LacksIslandwalk || kf.Flags2&kop.F2WithoutFlying == 0 {
		t.Fatalf("Pred %d PredN %#x Flags2 %#x, want PredWord7 / W7LacksIslandwalk beside the without-flying bit", kf.Pred, kf.PredN, kf.Flags2)
	}

	land, err := parseFilter("Island", func(w string) bool { return w == "Island" }, func(string) bool { return false })
	if err != nil {
		t.Fatal(err)
	}
	lk, err := cb.clause(land.Clauses[0], kop.ZoneBattlefield)
	if err != nil {
		t.Fatal(err)
	}
	if lk.Sub != 7 {
		t.Fatalf("Island landwalk clause Sub %d, want 7", lk.Sub)
	}
	lk.Sub, lk.Zone = 0, kop.ZoneNone
	if lk != (kop.Filter{}) {
		t.Errorf("Island landwalk clause holds more than the subtype: %+v", lk)
	}

	// the positive and other-parameter spellings are not read
	for _, w := range []string{"hasKeywordLandwalk:Island", "!hasKeywordLandwalk:Swamp", "withoutLandwalk"} {
		_, err := parseFilter("Creature."+w, func(s string) bool { return s == "Island" }, func(string) bool { return false })
		if err == nil || !strings.Contains(err.Error(), "unknown property") {
			t.Errorf("%s: err %v, want an unknown property", w, err)
		}
	}
}

// TestLacksIslandwalkLowers: a static CantAttack whose ValidCard$ carries the
// word lowers with no refusal.
func TestLacksIslandwalkLowers(t *testing.T) {
	const script = `Name:Islandwalk Gate Probe
ManaCost:3 U
Types:Creature Probe
PT:2/2
S:Mode$ CantAttack | ValidCard$ Creature.withoutFlying+!hasKeywordLandwalk:Island | Description$ probe gate
Oracle:probe
`
	if rf := lowerScript(t, "Islandwalk Gate Probe", script); len(rf) != 0 {
		t.Fatalf("refused: %v", rf)
	}
}
