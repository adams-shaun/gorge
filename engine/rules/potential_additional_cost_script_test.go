package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialAdditionalCostPlanReachesCensusSearch pins the composed-cost
// fallback for a plain cast the V1 planner declines for the shape of its
// additional cost. Champions of the Perfect's BeholdExile<1/Elf> RaiseCost is
// not part of the mana-only witness, but its {3}{G} is payable from three
// Forests and a Springleaf Drum tapping a creature -- a source the census
// cannot price. The fallback plan must report "insufficient" rather than the
// planner's shape refusal, so PotentialPaymentPlans' census/prefix search runs
// and reaches the Drum (CR 601.2b, 601.2g). Before that, the play was listed
// "unsupported shape:additional_cost" with no script and the lockstep kernel
// offered the cast gorge's vocabulary never had.
func TestPotentialAdditionalCostPlanReachesCensusSearch(t *testing.T) {
	t.Parallel()
	e, _ := raiseEngine(t, []string{"Champions of the Perfect", "Selfless Safewright", "Safewright Cavalry", "Springleaf Drum", "Forest", "Grizzly Bears"}, nil)
	champ := blightMove(t, e, 0, "Champions of the Perfect", state.ZHand)
	elf := blightMove(t, e, 0, "Selfless Safewright", state.ZHand)
	drum := blightMove(t, e, 0, "Springleaf Drum", state.ZBattlefield)
	blightMove(t, e, 0, "Safewright Cavalry", state.ZBattlefield)
	blightMove(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	for i := 0; i < 3; i++ {
		blightMove(t, e, 0, "Forest", state.ZBattlefield)
	}
	// Preconditions: the board can pay only by tapping the Drum, and the
	// behold candidate is where the additional-cost gate looks.
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("precondition: pool = %d, want 0 (the plan must tap sources)", got)
	}
	if o := e.G.Obj(elf); o == nil || o.Zone != state.ZHand || o.Face() == nil || !o.Face().IsCreature() {
		t.Fatalf("precondition: behold candidate %d is not a creature card in hand", elf)
	}
	if o := e.G.Obj(drum); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("precondition: Springleaf Drum %d is not an untapped battlefield permanent", drum)
	}
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "cast" || pp.Action.Obj != champ {
			continue
		}
		if pp.Reason != "" {
			t.Fatalf("Champions of the Perfect priced %q (%q), want a witness or script", pp.Reason, pp.Detail)
		}
		if len(pp.Script) == 0 {
			t.Fatal("Champions of the Perfect has no plan and no script: the census/prefix search did not reach Springleaf Drum")
		}
		return
	}
	t.Fatalf("Champions of the Perfect is not a potential play at all: %+v", e.PotentialPaymentPlans(0))
}
