package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"testing"
)

// TestTargetPaidPlanSettlesUnderAManaConvertRider: a planned cast whose only
// payment is its own target (a Treasure) is priced at its cheapest target set
// ({R} after the artifact-target reduction) and planned under the pip rider of
// a "spend mana as though it were mana of any type" static (CR 609.4b), so
// the witness cost is rider-generic {1}. The settle check re-derives the
// target-priced cost; it must apply the same rider, or the plan the engine
// offers fails its own ValidateCastPayment ("payment witness does not
// settle").
func TestTargetPaidPlanSettlesUnderAManaConvertRider(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 9413, ppFiller)
	spell := handCard(e, card(t, "Name:Gem Breaker rider\nManaCost:1 R\nTypes:Instant\n"+
		"A:SP$ Destroy | Cost$ 1 R | ValidTgts$ Artifact.Treasure | TgtPrompt$ Choose a gem\nOracle:x\n"), 0)
	onBoard(t, e, 0, tpTreasure)
	onBoard(t, e, 0, "Name:Gem Discount\nTypes:Enchantment\nS:Mode$ ReduceCost | ValidCard$ Card | Type$ Spell | Activator$ You | ValidTarget$ Artifact | Amount$ 1 | Description$ Spells you cast that target an artifact cost less.\nOracle:x\n")
	onBoard(t, e, 0, "Name:Gem Vizier\nTypes:Enchantment\nS:Mode$ ManaConvert | ValidPlayer$ You | ManaConversion$ AnyType->AnyType | Description$ Fixture: spend mana as though it were mana of any type.\nOracle:x\n")
	e.paidManaCensus = true
	ppPriority(t, e)
	e.EnsurePaymentActions()
	d := e.Pending()
	if d == nil || !ppHasAction(d, spell) {
		t.Fatal("precondition: a planned cast for the spell whose only target is the gem it pays with")
	}
	act := ppAction(t, d, spell)
	plan := act.Plans[0]
	if plan.Cost.Generic != 1 || plan.Cost.Mana != (decision.PaymentCost{}).Mana {
		t.Fatalf("precondition: witness cost %+v must be the rider-generic {1} (a kept coloured pip would not exercise the rider)", plan.Cost)
	}
	if err := e.ValidateCastPayment(0, act.Cast, plan); err != nil {
		t.Fatalf("the offered plan fails its own validation: %v", err)
	}
}
