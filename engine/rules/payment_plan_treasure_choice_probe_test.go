package rules

// The payment plan's sacrifice choice beside an enchanted Treasure. The
// lockstep contract holds the two planners to the same node-for-node choice
// (CR 601.2b prices the payment; which legal payment either planner finds is
// the rank), so for {3} paid from Tainted Field, a Swamp and three Treasures,
// one of which carries Relic Putrescence, the plan sacrifices the enchanted
// Treasure -- gorge's plan search returns the first acceptable plan, in
// battlefield zone order, and the enchanted one is the first of the three --
// and for {3}{W}, which needs a second Treasure, the enchanted one is still
// the first sacrifice. The kernel mirrors this by stamping the poison row
// ordinary (dirac/kgen lower_tapsclass.go tapsPoison, its plan key 8 then
// takes the battlefield-first unit), and the end-to-end parity is the oracle
// recording treasure_payment_choice. The sacrifice also sweeps the Aura whose
// host it was into its owner's graveyard (CR 704.5f).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestTreasurePaymentChoiceParityProbe(t *testing.T) {
	t.Parallel()
	sacrifices := func(t *testing.T, e *Engine, spell state.ObjID) []state.ObjID {
		t.Helper()
		got := e.PlanCastPayment(0, paymentCast(spell))
		if got.Plan == nil {
			t.Fatalf("no plan (reason %q detail %q), want the enchanted Treasure priced", got.Reason, got.Detail)
		}
		var sacs []state.ObjID
		for _, a := range got.Plan.Activations {
			if a.Consequence != nil && a.Consequence.Sacrifice {
				sacs = append(sacs, a.Source)
			}
		}
		if err := e.ValidateCastPayment(0, paymentCast(spell), *got.Plan); err != nil {
			t.Fatalf("ValidateCastPayment: %v", err)
		}
		return sacs
	}
	e, _, spell := newFixtureDeck(t, 77101, lrSpell("3"))
	t1, _, _ := pttEnchantedTreasure(t, e)
	if sacs := sacrifices(t, e, spell); len(sacs) != 1 || sacs[0] != t1 {
		t.Fatalf("{3}: plan sacrifices %v, want the enchanted Treasure %d first", sacs, t1)
	}
	e2, _, spell2 := newFixtureDeck(t, 77102, lrSpell("3 W"))
	w1, w2, _ := pttEnchantedTreasure(t, e2)
	if sacs := sacrifices(t, e2, spell2); len(sacs) != 2 || sacs[0] != w1 || sacs[1] != w2 {
		t.Fatalf("{3}{W}: plan sacrifices %v, want the enchanted Treasure %d first and %d next", sacs, w1, w2)
	}
}
