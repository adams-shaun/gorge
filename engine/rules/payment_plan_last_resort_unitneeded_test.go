package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// A {B}{R}{G} cost with a Swamp, a Tinder Wall and an Overgrown Battlement (three
// creatures with defender beside the Wall of Roots) is payable: each of the three
// covers a pip no other source does, though together they make six mana. The
// dirac kernel's planner agrees (TestEveryUnitNeededPlanIsFound).
func TestPaymentPlanLastResortEveryUnitNeeded(t *testing.T) {
	t.Parallel()
	e, _, spell := newFixtureDeck(t, 12031, lrSpell("B R G"))
	swamp := lrCorpus(t, e, "Swamp")
	wall := lrCorpus(t, e, "Tinder Wall")
	battlement := lrCorpus(t, e, "Overgrown Battlement")
	lrCorpus(t, e, "Wall of Roots")
	lrAssertPlan(t, e, e.PlanCastPayment(0, paymentCast(spell)), []lrStep{
		{swamp, decision.ManaAmount{0, 0, 1, 0, 0, 0}, nil},
		{battlement, decision.ManaAmount{0, 0, 0, 0, 3, 0}, nil},
		{wall, decision.ManaAmount{0, 0, 0, 2, 0, 0}, &decision.PaymentConsequence{Sacrifice: true}},
	})
}
