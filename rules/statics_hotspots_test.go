package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

var costStaticOptionsSink []decision.Option

// TestLegalActionsReusesCostStaticMembership catches a return to collecting
// the same immutable RaiseCost/ReduceCost/SetCost membership independently
// for every spell offer in one legal-actions pass. The Ability-only tax is
// deliberately inapplicable to these spells: it changes neither the options
// nor dynamic evaluation work, leaving repeated membership collection as the
// only per-spell allocation difference from the control game.
func TestLegalActionsReusesCostStaticMembership(t *testing.T) {
	spell := card(t, "Name:Probe\nManaCost:0\nTypes:Sorcery\nA:SP$ Draw | Defined$ You | NumCards$ 1\nOracle:x\n")
	hand := make([]*cards.Card, 12)
	for i := range hand {
		hand[i] = spell
	}

	control := handEngine(t, hand...)
	withStatic := handEngine(t, hand...)
	tax := card(t, "Name:Ability Tax\nManaCost:0\nTypes:Artifact\n"+
		"S:Mode$ RaiseCost | Type$ Ability | ValidCard$ Card | Amount$ 1\nOracle:x\n")
	o := withStatic.G.AddObject(tax, 0)
	o.Zone = state.ZBattlefield
	withStatic.G.SetZone(state.ZBattlefield, 0, []state.ObjID{o.ID})

	if got := kinds(control.legalActions(0))["cast"]; got != len(hand) {
		t.Fatalf("control cast options = %d, want %d", got, len(hand))
	}
	if got := kinds(withStatic.legalActions(0))["cast"]; got != len(hand) {
		t.Fatalf("ability-only tax changed spell offers: got %d, want %d", got, len(hand))
	}

	controlAllocs := allocsWithoutWalkCacheVerify(100, func() {
		costStaticOptionsSink = control.legalActions(0)
	})
	staticAllocs := allocsWithoutWalkCacheVerify(100, func() {
		costStaticOptionsSink = withStatic.legalActions(0)
	})
	if extra := staticAllocs - controlAllocs; extra > 3 {
		t.Fatalf("one cost static added %.0f allocations per legal-actions pass; want at most 3 (one call-scoped collection)", extra)
	}
}
