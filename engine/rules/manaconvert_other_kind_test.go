package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestManaConvertNeitherSideWidensOtherPayment: a ManaConvert static scoped
// ValidSA$ Activated (Agatha's Soul Cauldron's shape) or ValidSA$ Spell (Vizier
// of the Menagerie's shape) widens only that side's payments. A triggered,
// upkeep, echo, ward or unless payment is neither a spell cast nor an
// activation (CR 609.4b), so the offer gates (CostPayableOther, the explicit-pool
// CostPayablePool) and the charge (PayManaConv over Conv(SAOther)) must all
// refuse an {R} payment from a pool holding only {G}.
func TestManaConvertNeitherSideWidensOtherPayment(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, validSA string
		side          pay.SAKind
		ability       bool
	}{
		{"activated", "Activated", pay.SAActivated, true},
		{"spell", "Spell", pay.SASpell, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := layerEngine(t)
			onBoard(t, e, 0, "Name:Converter\nTypes:Artifact\nS:Mode$ ManaConvert | ValidPlayer$ You | ValidCard$ Creature.YouCtrl | ValidSA$ "+
				tc.validSA+" | ManaConversion$ AnyType->AnyType | Description$ x\nOracle:x\n")
			goo := onBoard(t, e, 0, "Name:Red Upkeeper\nManaCost:R\nTypes:Creature\nPT:2/2\nOracle:x\n")
			e.G.Players[0].Pool = state.Mana{}
			e.G.Players[0].Pool[state.MG] = 1
			red := ParseCost("R")
			// Precondition: the static reaches this creature on its own side, so
			// the refusals below are the side scoping, not a missed ValidCard$.
			if c := asEval(e).Conv(0, goo, tc.side); c == nil {
				t.Fatalf("precondition: the %s-side conversion does not reach the creature", tc.validSA)
			}
			if !pay.CostPayable(asPayer(e), 0, goo, tc.ability, red) {
				t.Fatalf("precondition: {R} is not payable from {G} on the %s side", tc.validSA)
			}
			if c := asEval(e).Conv(0, goo, pay.SAOther); c != nil {
				t.Errorf("Conv(SAOther) = %+v, want nil: a ValidSA$ %s static widened a non-spell, non-activated payment", c, tc.validSA)
			}
			if pay.CostPayableOther(asPayer(e), 0, goo, red) {
				t.Errorf("CostPayableOther offers {R} from {G} under a ValidSA$ %s conversion", tc.validSA)
			}
			if pay.CostPayablePool(asPayer(e), 0, goo, pay.SAOther, red, e.G.Players[0].Pool, e.G.Players[0].ManaUnits()) {
				t.Errorf("CostPayablePool(SAOther) offers {R} from {G} under a ValidSA$ %s conversion", tc.validSA)
			}
			if pay.PayManaConv(asPayer(e), 0, red, asEval(e).Conv(0, goo, pay.SAOther)) {
				t.Errorf("the charge paid {R} from {G} under a ValidSA$ %s conversion", tc.validSA)
			}
		})
	}
}
