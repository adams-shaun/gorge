package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// CR 601.2g with CR 605.3a: a mana ability whose whole cost taps another
// permanent (Urza, Lord High Artificer's "Tap an untapped artifact you
// control: Add {U}") is activated once per N distinct untapped candidates,
// so its production is not bounded from above by one: Urza beside two
// untapped artifacts funds {U}{U}, and beside six funds his own {5}. The
// potential bound counted the production once and withheld every play a
// second activation funds before any search ran -- the kernel priced the
// same board with a clone-DFS search over the tap-another sources and
// offered the play (the tail-64 Genju of the Spires pair the evidence loop
// parted on, decision 642).
func TestTapAnotherRepeatFundsMultiActivations(t *testing.T) {
	e := boundBoard(t, []string{
		"Urza, Lord High Artificer", "Jeweled Lotus",
		"Accorder's Shield", "Accorder's Shield", "Accorder's Shield", "Accorder's Shield",
	}, nil)
	var urza state.ObjID
	untapped := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		if !o.Tapped {
			untapped++
		}
		if o.Face().Name == "Urza, Lord High Artificer" {
			urza = id
		}
	}
	if urza == 0 {
		t.Fatal("precondition: Urza is not on the battlefield")
	}
	if untapped < 6 {
		t.Fatalf("precondition: six untapped artifacts beside Urza, got %d", untapped)
	}
	_, listed := e.potentialWalkOf(0, true)
	// the bound's own output: the potential walk lists Urza's {5} activation
	// (addPotentialTapRepeat folds the repeats into the bound; the plan below
	// is the payment layer, which prices the repeats on its own)
	walked := false
	for _, o := range listed {
		if o.Obj == urza && o.Kind == "ability" {
			walked = true
		}
	}
	if !walked {
		t.Fatalf("the potential walk withholds Urza's {5} activation the six tap-another repeats fund: %+v", listed)
	}
	plan, ok := PotentialPlan{}, false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == urza && pp.Action.Ability == 1 {
			plan, ok = pp, true
		}
	}
	if !ok || plan.Reason != "" || len(plan.Script) == 0 {
		t.Fatalf("Urza's {5} is payable through six tap-another activations: listed %v plan %+v reason %q detail %q script %d",
			listed, plan.Action, plan.Reason, plan.Detail, len(plan.Script))
	}
}
