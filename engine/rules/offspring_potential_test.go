package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The potential planner prices an offspring cast as base plus offspring, so a play
// that taps its lands can pay the optional cost. Without it the mode was reachable
// only by floating the whole cost by hand, which no surface but a human's took.
func TestPotentialPlanPricesOffspring(t *testing.T) {
	t.Parallel()
	const src = "Name:Probe Offspring\nManaCost:1 G\nTypes:Creature Rabbit\nPT:1/1\nK:Offspring:2\nOracle:x\n"
	e := handEngine(t, card(t, src))
	id := e.G.Zone(state.ZHand, 0)[0]
	got := func(p int32) PaymentPlanOutcome {
		e.G.Players[0].Pool[state.MG] = p
		return e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: id, Mode: "offspring"})
	}
	if out := got(3); out.Reason != "insufficient" {
		t.Fatalf("three mana for a four-mana offspring cast: %+v, want insufficient", out)
	}
	if out := got(4); out.Reason != "" {
		t.Fatalf("four mana for a four-mana offspring cast: %+v, want a plan", out)
	}
	if out := e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: id, Mode: "buyback"}); out.Reason == "" {
		t.Fatalf("a mode the card lacks was priced: %+v", out)
	}
}
