package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// faceTaxSwamp is an authored untapped Swamp source for the face-tax planner
// regression below.
const faceTaxSwamp = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"

// TestAdventureFaceUnderTaxOfferedFromUntappedLands pins the planner's
// alternate-face composition against the offer gate's: the Adventure face is
// priced with the object flipped to it, so a face-dependent tax (Thalia's
// ValidCard$ Card.nonCreature) applies to the witness exactly as the gate
// applies it. Before this the planner priced the Creature front ({1}{B}),
// the gate priced the Sorcery face ({2}{B}), and potentialWitnessOffers
// rejected the witness -- the cast was offered only from floating mana.
func TestAdventureFaceUnderTaxOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 7413, taxedAdventureSrc, taxWardenSrc)
	putCreature(t, e, 0, taxWardenSrc)
	var swamps []state.ObjID
	for range 5 {
		swamps = append(swamps, onBoard(t, e, 0, faceTaxSwamp))
	}
	toMain1(t, e)
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	ppPriority(t, e)
	pp := faceCastPlan(t, e.PotentialPaymentPlans(0), id, "adventure_alt")
	if pp.Plan == nil || pp.Reason != "" {
		t.Fatalf("taxed adventure_alt verdict %+v, want a witness from the untapped Swamps", pp)
	}
	// The taxed face is {2}{B}: three Swamps, not the front's two.
	if len(pp.Plan.Activations) != 3 {
		t.Fatalf("witness %s, want three Swamps", lrPlanString(pp.Plan.Activations))
	}
	for _, a := range pp.Plan.Activations {
		found := false
		for _, src := range swamps {
			found = found || a.Source == src
		}
		if !found {
			t.Fatalf("witness tapped %d, want only the untapped Swamps", a.Source)
		}
	}
}
