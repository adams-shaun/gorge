package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestBasePowerPredicateFamily is the truth table for the base-P/T filter
// predicates the corpus carries (13 files): the literal `basePower<CMP>N` /
// `baseToughness<CMP>N` family (Sword of the Squeak, Bess, Duskana) and the
// characteristic-vs-characteristic family `powerGTbasePower` /
// `powerNOTbasePower` (Baird, Sovereign Okinec Ahau, Kutzil). The candidate
// is a printed 2/2 Bear with a +1/+1 counter: its CURRENT power/toughness is
// 3/3 while its BASE stays 2/2, so the two spellings must disagree.
func TestBasePowerPredicateFamily(t *testing.T) {
	g, id := board(t)
	bear := g.Obj(id["myBear"])

	// Precondition: the compared values actually differ. A vacuous board
	// (counter absent) would let every case below pass for the wrong reason.
	bear.AddCounter("P1P1", 1)
	if objectPower(bear) != 3 || objectBasePower(bear) != 2 {
		t.Fatalf("precondition: bear current power %d base power %d, want 3 and 2",
			objectPower(bear), objectBasePower(bear))
	}
	if objectToughness(bear) != 3 || objectBaseToughness(bear) != 2 {
		t.Fatalf("precondition: bear current toughness %d base toughness %d, want 3 and 2",
			objectToughness(bear), objectBaseToughness(bear))
	}

	cases := []struct {
		spec string
		want bool
	}{
		// Literal base-power family: the counter does not move the base.
		{"Creature.basePowerEQ2", true},
		{"Creature.basePowerEQ3", false},
		{"Creature.basePowerLE2", true},
		{"Creature.basePowerGE2", true},
		{"Creature.basePowerLT2", false},
		{"Creature.basePowerGT1", true},
		// Literal base-toughness family.
		{"Creature.baseToughnessEQ2", true},
		{"Creature.baseToughnessEQ3", false},
		{"Creature.baseToughnessLE2", true},
		{"Creature.baseToughnessGT2", false},
		// Current-vs-base: the counter makes power (3) exceed base (2).
		{"Creature.powerGTbasePower", true},
		{"Creature.powerNOTbasePower", true},
		{"Creature.powerEQbasePower", false},
		{"Creature.toughnessGTbaseToughness", true},
		{"Creature.toughnessNOTbaseToughness", true},
		{"Creature.basePowerGTpower", false},
		// A negated spelling must invert, not fall closed.
		{"Creature.!powerGTbasePower", false},
		{"Creature.!basePowerEQ2", false},
	}
	for _, c := range cases {
		if got := MatchesSpec(g, c.spec, bear.ID, 0); got != c.want {
			t.Errorf("%s = %v, want %v", c.spec, got, c.want)
		}
	}
}

// TestBasePowerPredicateBoundContext proves the rules-bound layer bridge: a
// SpecContext that carries DerivedPower/DerivedToughness and
// BasePower/BaseToughness (the fields rules' matchesSpec sets from
// Engine.Derived) is authoritative, so a 7c pump (matched power above base)
// and a 7b set (base itself moved) are both read correctly even though the
// object's printed face is a vanilla 2/2. Without the binding the predicate
// falls back to the object-alone read, which is the direct-call path.
func TestBasePowerPredicateBoundContext(t *testing.T) {
	g, id := board(t)
	bear := g.Obj(id["myBear"])
	if objectBasePower(bear) != 2 || objectBaseToughness(bear) != 2 {
		t.Fatalf("precondition: unbound base P/T %d/%d, want 2/2",
			objectBasePower(bear), objectBaseToughness(bear))
	}

	// A 7c pump: current 5/5, base still 2/2.
	pumped := SpecContext{HasDerivedPT: true, DerivedPower: 5, DerivedToughness: 5,
		HasBasePT: true, BasePower: 2, BaseToughness: 2}
	if !MatchesSpecCtx(g, "Creature.powerGTbasePower", bear.ID, pumped) {
		t.Error("powersGTbasePower must match a 7c-pumped 5/5 with base 2/2")
	}
	if MatchesSpecCtx(g, "Creature.powerEQbasePower", bear.ID, pumped) {
		t.Error("powerEQbasePower must reject a 7c-pumped 5/5 with base 2/2")
	}

	// A 7b set: base itself is now 4/3 (Andrios' SetPower$ 16 style, at a
	// smaller size), current 4/3 with no 7c modify.
	set := SpecContext{HasDerivedPT: true, DerivedPower: 4, DerivedToughness: 3,
		HasBasePT: true, BasePower: 4, BaseToughness: 3}
	if !MatchesSpecCtx(g, "Creature.basePowerEQ4", bear.ID, set) {
		t.Error("basePowerEQ4 must match a 7b-set base power of 4")
	}
	if MatchesSpecCtx(g, "Creature.basePowerEQ2", bear.ID, set) {
		t.Error("basePowerEQ2 must reject a 7b-set base power of 4")
	}
	if !MatchesSpecCtx(g, "Creature.baseToughnessEQ3", bear.ID, set) {
		t.Error("baseToughnessEQ3 must match a 7b-set base toughness of 3")
	}
	if MatchesSpecCtx(g, "Creature.powerGTbasePower", bear.ID, set) {
		t.Error("powerGTbasePower must reject when current equals base")
	}

	// The unbound context (no HasBasePT/HasDerivedPT) keeps the object-alone
	// read: printed 2/2. This is the direct-call fallback, not the rules path.
	if !MatchesSpecCtx(g, "Creature.basePowerEQ2", bear.ID, SpecContext{}) {
		t.Error("an unbound context must still match the printed base power 2")
	}
}

// TestBasePowerPredicateUnknownCensus asserts every spelling the 13 corpus
// files carry is recognised by UnknownPredicates -- the census and the
// matcher share recognisedPredicate, so an unrecognised token would both
// fail closed at match time and show up here.
func TestBasePowerPredicateUnknownCensus(t *testing.T) {
	specs := []string{
		"Creature.basePowerEQ0", "Creature.basePowerEQ1", "Creature.basePowerEQ2",
		"Creature.basePowerEQ4", "Creature.basePowerLE1",
		"Creature.baseToughnessEQ1", "Creature.baseToughnessEQ2",
		"Creature.baseToughnessEQ3", "Creature.baseToughnessLE1",
		"Creature.YouCtrl+powerGTbasePower",
		"Creature.YouCtrl+powerNOTbasePower",
		"Creature.basePowerEQ1+baseToughnessEQ1+Other+YouCtrl",
	}
	for _, spec := range specs {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want none", spec, un)
		}
	}
}

// TestBasePowerPredicateRejectsUnboostedPeer is the negative-candidate half:
// the RHS forms select the boosted object and reject its unboosted peer, and
// the literal forms do the same on base toughness. Both creatures are printed
// 2/2, but only one carries the counter.
func TestBasePowerPredicateRejectsUnboostedPeer(t *testing.T) {
	g, id := board(t)
	boosted := g.Obj(id["myBear"])
	boosted.AddCounter("P1P1", 2) // current 4/4, base 2/2
	peer := g.Obj(id["myFlier"])
	// Precondition: the peer is the unboosted candidate, and the two current
	// powers really differ.
	if objectPower(boosted) != 4 || objectPower(peer) != 1 {
		t.Fatalf("precondition: boosted power %d peer power %d, want 4 and 1",
			objectPower(boosted), objectPower(peer))
	}
	if objectBasePower(boosted) != 2 || objectBasePower(peer) != 1 {
		t.Fatalf("precondition: boosted base %d peer base %d, want 2 and 1",
			objectBasePower(boosted), objectBasePower(peer))
	}

	if !MatchesSpec(g, "Creature.powerGTbasePower", boosted.ID, 0) {
		t.Error("the boosted creature must match powerGTbasePower")
	}
	// The Flier is 1/1 with no counter, so power == base: it must be rejected.
	if MatchesSpec(g, "Creature.powerGTbasePower", peer.ID, 0) {
		t.Error("an unboosted 1/1 must not match powerGTbasePower")
	}
	if MatchesSpec(g, "Creature.powerNOTbasePower", peer.ID, 0) {
		t.Error("an unboosted 1/1 must not match powerNOTbasePower")
	}
	// The literal base form selects by base, ignoring the counter: the boosted
	// 4/4 still has base power 2.
	if !MatchesSpec(g, "Creature.basePowerEQ2", boosted.ID, 0) {
		t.Error("a countered base-2 creature must still match basePowerEQ2")
	}
	if MatchesSpec(g, "Creature.basePowerEQ4", boosted.ID, 0) {
		t.Error("a countered base-2 creature must not match basePowerEQ4 (current is 4)")
	}
}

// TestBasePowerPredicateNOTOperator pins the three-character NOT operator
// alone so a future refactor of the numeric-RHS split cannot silently fall
// back to the two-character slice (which read `powerNOTbasePower` as cmp
// "NO" plus the unparseable RHS "TbasePower" and returned never-match).
func TestBasePowerPredicateNOTOperator(t *testing.T) {
	if _, ok := numericPred("powerNOTbasePower", nil, &state.Object{}, SpecContext{}); !ok {
		t.Fatal("powerNOTbasePower is not recognised by numericPred")
	}
	if _, ok := numericPred("powerGTbasePower", nil, &state.Object{}, SpecContext{}); !ok {
		t.Fatal("powerGTbasePower is not recognised by numericPred")
	}
	if _, ok := numericPred("basePowerEQ1", nil, &state.Object{}, SpecContext{}); !ok {
		t.Fatal("basePowerEQ1 is not recognised by numericPred")
	}
	if _, ok := numericPred("baseToughnessLE1", nil, &state.Object{}, SpecContext{}); !ok {
		t.Fatal("baseToughnessLE1 is not recognised by numericPred")
	}
	// An unknown comparison operator still fails closed -- never a match on a
	// real object -- so a typo cannot widen a filter.
	g, id := board(t)
	if MatchesSpec(g, "Creature.powerZZ3", id["myBear"], 0) {
		t.Fatal("powerZZ3 has no valid operator and must never match")
	}
	if un := UnknownPredicates("Creature.powerNOTbasePower"); len(un) != 0 {
		t.Fatalf("UnknownPredicates(powerNOTbasePower) = %v", un)
	}
}
