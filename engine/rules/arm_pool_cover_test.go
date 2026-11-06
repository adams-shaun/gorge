package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The consumption-only arm's pool-cover pre-gate (planPoolCoversPrintedCost)
// proves the arm's verdict false without the census pass or the plan whenever
// the floating pool alone pays the composed printed cost: the zero-source
// plan the search then returns consumes nothing, so the cast is not payable
// only by paying with a permanent its own announcements need. The assertions
// pin the pre-gate's own answers -- coverage, one-short, empty pool, and an
// announced X, whose payable value is the planner's X search's answer and
// never the printed cost's -- and, through the arm, that the verdict on a
// pool-covered shape cast is still false and plans no payment.
func TestArmPoolCoverDeclinesPlannedPayment(t *testing.T) {
	t.Parallel()
	te, _, _ := newFixtureDeck(t, 9415, ppFiller)
	spell := handCard(te, card(t, acoSpell), 0)
	// A forest is the only source on the battlefield: the printed {W} cannot
	// come from the board, so only the floating pool can pay.
	onBoard(t, te, 0, acoForest)
	te.G.Step = state.StepMain1
	ppPriority(t, te)
	if detail := te.paymentPlanCastShapeDetail(0, spell); detail != "shape:target_dependent_cost" {
		t.Fatalf("shape detail %q, want shape:target_dependent_cost", detail)
	}
	opt := decision.Option{Kind: "cast", Obj: spell}
	te.G.Players[0].Pool[state.MW] = 5
	if !te.planPoolCoversPrintedCost(0, opt) {
		t.Fatal("the floating pool does not cover the printed {4}{W}")
	}
	if te.targetPaidNeedsPlannedPayment(0, spell) {
		t.Fatal("a pool-covered cast is not payable only by paying with a needed permanent")
	}
	te.G.Players[0].Pool[state.MW] = 4
	if te.planPoolCoversPrintedCost(0, opt) {
		t.Fatal("four floating mana does not cover {4}{W}")
	}
	te.G.Players[0].Pool[state.MW] = 0
	if te.planPoolCoversPrintedCost(0, opt) {
		t.Fatal("an empty pool does not cover the printed cost")
	}

	// The X spell's payable value is the planner's X search's answer, never
	// the printed cost's: the pre-gate refuses it whatever the pool holds.
	te2, _, _ := newFixtureDeck(t, 9416, ppFiller)
	xspell := handCard(te2, card(t, "Name:Arm Consumption X\nManaCost:X R\nTypes:Instant\n"+
		"A:SP$ Damage | Tgt$ TgtPlayer | SpellDescription$ It deals X damage. | Oracle:x\n"), 0)
	te2.G.Step = state.StepMain1
	ppPriority(t, te2)
	te2.G.Players[0].Pool[state.MW] = 1
	te2.G.Players[0].Pool[state.MR] = 20
	if te2.planPoolCoversPrintedCost(0, decision.Option{Kind: "cast", Obj: xspell}) {
		t.Fatal("an announced X is not proven by the printed cost's pool cover")
	}

	// End to end: the pool-covered shape cast still carries no planned
	// payment -- the same offer the plain-source cast gets.
	te.G.Players[0].Pool[state.MW] = 5
	te.EnsurePaymentActions()
	if d := te.Pending(); ppHasAction(d, spell) {
		t.Fatal("a planned payment was offered for a pool-covered cast")
	}
}
