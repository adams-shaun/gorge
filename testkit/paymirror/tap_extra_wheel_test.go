package paymirror

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPreferLiftedFlagsATapsForManaExtraWheel pins the trigger that widens
// answerManaAsks' clone proof. The plan's own printed ability is offered as a
// wheel option whose label production is LESS than the witness, because a
// TapsForMana extra (Forsaken Monument's "whenever you tap a permanent for
// {C}, add an additional {C}") lifts the tapped ability's true production
// above its label. A label match on a DIFFERENT option is then a false
// positive: paymirror seed 6085 taps Eldrazi Temple, whose
// {T}: Add {C} plus the extra reads C:2 -- exactly the label of its
// {T}: Add {C}{C} -- so a production-only match tapped Add {C}{C} and
// over-produced (production_differs). preferLifted routes that wheel through
// verifiedProductions, which proves the plan's ability on a clone.
func TestPreferLiftedFlagsATapsForManaExtraWheel(t *testing.T) {
	cc := decision.ManaAmount{}
	cc[state.ManaIndex('C')] = 2
	c1 := decision.ManaAmount{}
	c1[state.ManaIndex('C')] = 1
	d := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "mana", Ability: 0, Label: "Add C"},
			{Index: 1, Kind: "mana", Ability: 1, Label: "Add CC"},
		}}

	// The Add CC label names the C:2 witness exactly: labelExact is the
	// false positive the fix routes around, and matchProductions alone picks
	// that wrong option even though the plan named Add C (rank 0).
	if !labelExact(d, cc) {
		t.Fatal("the Add CC label must match the C:2 witness exactly")
	}
	if got := matchProductions(d, cc, 0); len(got) == 0 || got[0] != 1 {
		t.Fatalf("matchProductions without the proof = %v, want the Add CC option first", got)
	}

	// The plan's printed ability (rank 0, Add C) is offered but its label is
	// one short: the extra can lift it, so the clone proof must run.
	if !preferLifted(d, 0, cc) {
		t.Fatal("the plan's Add C option must be flagged as label-understated")
	}
	// The Add CC option's own label equals the witness: not lifted.
	if preferLifted(d, 1, cc) {
		t.Fatal("the Add CC label equals the witness; must not be lifted")
	}
	// No printed ability rank: nothing to lift.
	if preferLifted(d, -1, cc) {
		t.Fatal("no printed ability rank must not be lifted")
	}
	// The plan's ability's label equals the witness: not lifted.
	if preferLifted(d, 0, c1) {
		t.Fatal("the Add C label equals a C:1 witness; must not be lifted")
	}
}
