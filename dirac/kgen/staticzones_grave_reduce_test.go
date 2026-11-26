package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestStaticZonesGraveReduceForms: Convergence of Dominion's "activated
// abilities of cards in your graveyard cost {2} less, to a minimum of one
// mana" (ReduceCost Type$ Ability, AffectedZone$ Graveyard, MinMana$ 1) lowers
// to a StatReduceAbility static on the graveyard zone, with the MinMana$ floor
// in its B bits.
func TestStaticZonesGraveReduceForms(t *testing.T) {
	c := lowerNamedCard(t, "Convergence of Dominion")
	if len(c.Refusals) != 0 {
		t.Fatalf("Convergence of Dominion refusals = %v, want none", c.Refusals)
	}
	var got *lStatic
	for i := range c.Statics {
		if c.Statics[i].Kind == "ReduceCost" && c.Statics[i].AbilityRed != 0 {
			got = &c.Statics[i]
		}
	}
	if got == nil {
		t.Fatalf("no ability ReduceCost static among %+v", c.Statics)
	}
	if got.Zone != int(zGraveyard) {
		t.Errorf("static zone = %d, want the graveyard (%d)", got.Zone, zGraveyard)
	}
	if got.MinMana != 1 {
		t.Errorf("MinMana = %d, want 1", got.MinMana)
	}
	b, _ := reduceAbBits(*got)
	if int(b>>kop.ReduceAbMinShift)&15 != 1 {
		t.Errorf("B floor bits = %d, want 1", int(b>>kop.ReduceAbMinShift)&15)
	}
}
