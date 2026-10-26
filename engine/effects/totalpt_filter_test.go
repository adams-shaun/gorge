package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTotalPTFilter pins Forge's totalPT card property: current power plus
// current toughness, compared with the numeric operators. Cut Down
// (Creature.totalPT_LE5) is the corpus carrier. The property is spelled with
// an underscore before the comparator, unlike power/toughness/cmc.
func TestTotalPTFilter(t *testing.T) {
	g, id := board(t)
	mk := func(owner state.PlayerID, src string) state.ObjID {
		o := g.AddObject(mkCard(t, src), owner)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, owner, append(g.Zone(state.ZBattlefield, owner), o.ID))
		return o.ID
	}
	twoThree := mk(1, "Name:TwoThree\nTypes:Creature Test\nPT:2/3\nOracle:x\n")
	threeThree := mk(1, "Name:ThreeThree\nTypes:Creature Test\nPT:3/3\nOracle:x\n")
	pumped := mk(1, "Name:Pumped\nTypes:Creature Test\nPT:2/3\nOracle:x\n")

	// Preconditions: the candidates really carry the P/T the rule reads, and
	// the non-creature really has none.
	if p, tt := objectPower(g.Obj(twoThree)), objectToughness(g.Obj(twoThree)); p != 2 || tt != 3 {
		t.Fatalf("precondition: 2/3 candidate is %d/%d", p, tt)
	}
	if p, tt := objectPower(g.Obj(threeThree)), objectToughness(g.Obj(threeThree)); p != 3 || tt != 3 {
		t.Fatalf("precondition: 3/3 candidate is %d/%d", p, tt)
	}
	if !lacksPT(g.Obj(id["myLand"]), SpecContext{}) {
		t.Fatal("precondition: the land must have no P/T")
	}

	cases := []struct {
		name string
		spec string
		obj  state.ObjID
		want bool
	}{
		// 2/3 total 5: LE5 matches, a 3/3 (total 6) does not.
		{"2/3 total 5 is LE5", "Creature.totalPT_LE5", twoThree, true},
		{"3/3 total 6 is not LE5", "Creature.totalPT_LE5", threeThree, false},
		// No P/T: no totalPT comparison can hold (same rule as power).
		{"non-creature has no totalPT", "Permanent.totalPT_LE5", id["myLand"], false},
		// GE and EQ.
		{"2/3 total 5 is GE5", "Creature.totalPT_GE5", twoThree, true},
		{"3/3 total 6 is GE6", "Creature.totalPT_GE6", threeThree, true},
		{"2/3 total 5 is not GE6", "Creature.totalPT_GE6", twoThree, false},
		{"2/3 total 5 is EQ5", "Creature.totalPT_EQ5", twoThree, true},
		{"3/3 total 6 is not EQ5", "Creature.totalPT_EQ5", threeThree, false},
		{"3/3 total 6 is EQ6", "Creature.totalPT_EQ6", threeThree, true},
	}
	for _, c := range cases {
		if got := MatchesSpec(g, c.spec, c.obj, 0); got != c.want {
			t.Errorf("%s: MatchesSpec(%q) = %v, want %v", c.name, c.spec, got, c.want)
		}
	}

	// A layer-7 pump changes the answer: the same 2/3 body, bound as 4/5 by
	// the rules layer walk, totals 9 and no longer matches LE5 (but matches
	// GE9). The bind is how rules' matchesSpec hands the evaluator a pump.
	if !MatchesSpec(g, "Creature.totalPT_LE5", pumped, 0) {
		t.Fatal("precondition: the unpumped 2/3 must match totalPT_LE5")
	}
	pumpCtx := SpecContext{HasDerivedPT: true, DerivedPower: 4, DerivedToughness: 5}
	if MatchesSpecCtx(g, "Creature.totalPT_LE5", pumped, pumpCtx) {
		t.Error("a layer-7-pumped 4/5 (total 9) must not match totalPT_LE5")
	}
	if !MatchesSpecCtx(g, "Creature.totalPT_GE9", pumped, pumpCtx) {
		t.Error("a layer-7-pumped 4/5 (total 9) must match totalPT_GE9")
	}

	// The object-alone read (no bind) sees a layer-7d counter too.
	g.Obj(pumped).AddCounter("P1P1", 1) // 3/4, total 7
	if MatchesSpec(g, "Creature.totalPT_LE5", pumped, 0) {
		t.Error("a +1/+1-countered 2/3 (total 7) must not match totalPT_LE5")
	}
	if !MatchesSpec(g, "Creature.totalPT_EQ7", pumped, 0) {
		t.Error("a +1/+1-countered 2/3 must total 7")
	}

	// The census and the matcher share recognisedPredicate: the Cut Down
	// spelling must be known, or the card is refused before it can target.
	for _, spec := range []string{
		"Creature.totalPT_LE5",
		"Creature.totalPT_GE5",
		"Creature.totalPT_EQ5",
		"Creature.YouCtrl+totalPT_LE5",
	} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
	if !SpecReadsPT("Creature.totalPT_LE5") {
		t.Error("SpecReadsPT must report totalPT so the rules seam binds derived P/T")
	}
}
