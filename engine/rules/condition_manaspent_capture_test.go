package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestConditionManaSpentCaptureOnVigorMortis casts the real corpus Vigor
// Mortis (a top-level ConditionManaNotSpent$ G gate plus a ConditionManaSpent$
// G SVar body) paid with GGBB and asserts the per-colour spend vector is
// recorded on the stack object, so the condition evaluator has something to
// read.
func TestConditionManaSpentCaptureOnVigorMortis(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Vigor Mortis"))
	lion := e.G.AddObject(corpusAlternativeCard(t, "Silvercoat Lion"), 0)
	lion.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, []state.ObjID{lion.ID})
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MG], e.G.Players[0].Pool[state.MB] = 2, 2
	castMode(t, e, id, "")
	submitTarget(t, e, lion.ID)
	o := e.G.Obj(id)
	if o.Zone != state.ZStack {
		t.Fatalf("precondition: zone=%s, want stack", o.Zone)
	}
	if o.ManaColorSpent[state.MG] <= 0 {
		t.Fatalf("ManaColorSpent = %v, want green spend recorded", o.ManaColorSpent)
	}
	if o.CastFlags&state.FlagManaColorSpent == 0 {
		t.Fatalf("CastFlags = %d, want FlagManaColorSpent set", o.CastFlags)
	}
}

// TestConditionManaSpentPlannerGate asserts the payment planner declines a
// spell whose TOP-LEVEL ability carries a mana-spent condition clause (a
// parsed param key, not an SVar body), leaving the colours to the caster.
func TestConditionManaSpentPlannerGate(t *testing.T) {
	t.Parallel()
	for i, clause := range []string{"ConditionManaSpent$ G", "ConditionManaNotSpent$ G"} {
		t.Run(clause, func(t *testing.T) {
			t.Parallel()
			src := "Name:Shape Gate Spell\nManaCost:B\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | " + clause + "\nOracle:test\n"
			e, _, spell := newFixtureDeck(t, uint64(9700+i), src)
			onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:test\n")
			got := e.PlanCastPayment(0, paymentCast(spell))
			if got.Plan != nil || got.Reason != "unsupported" || got.Detail != "shape:mana_spent_reader" {
				t.Fatalf("plan outcome = %+v, want unsupported shape:mana_spent_reader", got)
			}
		})
	}
}
