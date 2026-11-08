package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialPlanNamedRaiseCostIsNotProvenUnpayable pins the line between a
// fixed RaiseCost extra, whose composed mana price is exact, and a
// named-announcement one (Explosive Singularity's tapXType<@Tapped>), which
// potentialModeCastPlan composes at announcement 0 while the offer gate sweeps
// the legal counts. Five lands' mana and five untapped creatures make the cast
// cost {3}{R}{R} at the announcement of five taps, so the raw priority options
// offer it; the planner must not prove it unpayable from the announcement-0
// composition.
func TestPotentialPlanNamedRaiseCostIsNotProvenUnpayable(t *testing.T) {
	t.Parallel()
	e, _ := raiseEngine(t, []string{"Explosive Singularity"}, nil)
	spell := blightMove(t, e, 0, "Explosive Singularity", state.ZHand)
	for i := 0; i < 5; i++ {
		blightMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	}
	addMana(t, e, 0, "RRRRR")
	ppPriority(t, e)

	// The precondition the assertion depends on: the engine itself offers
	// the cast, so no plan may call it unpayable.
	if !raiseCastOffered(e, spell) {
		t.Fatalf("precondition: Explosive Singularity not in the raw options at five mana and five untapped creatures: %+v", e.Pending().Options)
	}

	pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", spell)
	if pp.Reason == "insufficient" {
		t.Fatalf("plan reason insufficient (%s) for a cast the raw options offer: the named tap count prices the composed plan at announcement 0, which is no proof", pp.Detail)
	}

	// A host reads the same proof through EnsurePotentialProofs and
	// PotentialActions; it must not hide the cast the raw options offer.
	e.EnsurePotentialProofs()
	for _, a := range e.PotentialActions(0) {
		if a.Kind == "cast" && a.Obj == spell && a.Payable != nil && !*a.Payable {
			t.Fatal("PotentialActions marks the offered Explosive Singularity cast unpayable")
		}
	}
}
