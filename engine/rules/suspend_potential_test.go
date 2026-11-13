package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The suspend special action is witnessed by the potential walk like plot's:
// a card payable only by tapping a land is offered with a plan, so an
// autopay bot can suspend from untapped lands (CR 702.62a).
func TestSuspendIsWitnessedByThePotentialPlan(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, "Name:Suspender\nManaCost:3 R\nTypes:Creature Human\nPT:2/2\nK:Suspend:2:R\nOracle:x\n"))
	mtn := card(t, "Name:Mountain\nTypes:Basic Land Mountain\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n")
	land := e.G.AddObject(mtn, 0)
	land.Zone = state.ZBattlefield
	e.G.SetZone(state.ZBattlefield, 0, []state.ObjID{land.ID})
	e.G.Step, e.G.Active = state.StepMain1, 0
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "cast" && pp.Action.Mode == "suspend" {
			if pp.Reason != "" || pp.Plan == nil {
				t.Fatalf("the suspend action has no payment plan: reason %q detail %q", pp.Reason, pp.Detail)
			}
			return
		}
	}
	t.Fatalf("the potential walk lists no suspend action for a card payable by tapping the Mountain")
}
