package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// A second "Sacrifice another permanent: Add" source must never remove a way
// to pay (CR 601.2g-h): with two Ashnod's Altars and one creature, a cost one
// altar plus the lands funds, the planner must still return a plan -- the
// search's best may price both altars, but victim assignment then fails on
// the second step (one distinct victim), and the planner must look for a plan
// with fewer other-sacrifice steps instead of answering insufficient.

const (
	twoSacSpellSrc  = "Name:TwoSac Gyre\nManaCost:4 U U\nTypes:Instant\nA:SP$ Draw | NumCards$ 1 | SpellDescription$ x\nOracle:x\n"
	twoSacAltarSrc  = "Name:TwoSac Altar\nTypes:Artifact\nA:AB$ Mana | Cost$ Sac<1/Creature> | Produced$ C | Amount$ 2 | SpellDescription$ x\nOracle:x\n"
	twoSacIslandSrc = "Name:TwoSac Island\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ x\nOracle:x\n"
	twoSacPreySrc   = "Name:TwoSac Prey\nManaCost:1\nTypes:Creature\nPT:1/1\nOracle:x\n"
)

// twoSacBoard places altars altars, one creature and five untapped islands on
// seat 0's battlefield and returns the engine with the gyre in hand.
func twoSacBoard(t *testing.T, seed uint64, altars int) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, spell := newFixtureDeck(t, seed, twoSacSpellSrc)
	for i := 0; i < altars; i++ {
		onBoard(t, e, 0, twoSacAltarSrc)
	}
	prey := onBoardReady(t, e, 0, twoSacPreySrc)
	for i := 0; i < 5; i++ {
		onBoardReady(t, e, 0, twoSacIslandSrc)
	}
	// Preconditions: the victim is a battlefield creature the altars' Sac
	// part sees, and it is the board's only one.
	if e.G.Obj(prey).Zone != state.ZBattlefield || !e.IsCreature(prey) {
		t.Fatalf("precondition: prey zone %s creature %v, want a battlefield creature", e.G.Obj(prey).Zone, e.IsCreature(prey))
	}
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.IsCreature(id) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("precondition: %d creatures on seat 0's battlefield, want exactly one", n)
	}
	return e, spell, prey
}

func TestPaymentPlanTwoSacSourcesOneVictim(t *testing.T) {
	t.Parallel()
	t.Run("one altar pays", func(t *testing.T) {
		// The control: one other-sacrifice source funds the same cost, so the
		// fixture really is payable and a missing plan below is the second
		// source's doing, not an unfundable board.
		e, spell, prey := twoSacBoard(t, 4741, 1)
		got := e.PlanCastPayment(0, paymentCast(spell))
		if got.Plan == nil || got.Reason != "" {
			t.Fatalf("one-altar plan = reason %q detail %q plan %v, want a plan", got.Reason, got.Detail, got.Plan)
		}
		victims := twoSacVictims(got.Plan)
		if len(victims) != 1 || victims[0] != prey {
			t.Fatalf("one-altar victims = %v, want the prey %d named once", victims, prey)
		}
	})
	t.Run("second source keeps the plan", func(t *testing.T) {
		e, spell, prey := twoSacBoard(t, 4742, 2)
		got := e.PlanCastPayment(0, paymentCast(spell))
		if got.Plan == nil || got.Reason != "" {
			t.Fatalf("two-altar plan = reason %q detail %q, want a plan: a second other-sacrifice source must never remove a way to pay", got.Reason, got.Detail)
		}
		victims := twoSacVictims(got.Plan)
		if len(victims) != 1 || victims[0] != prey {
			t.Fatalf("two-altar victims = %v, want the prey %d named exactly once", victims, prey)
		}
	})
}

// twoSacVictims is plan's named other-sacrifice victims, in step order.
func twoSacVictims(plan *decision.PaymentPlan) []state.ObjID {
	var out []state.ObjID
	for _, a := range plan.Activations {
		if a.Consequence != nil && a.Consequence.SacrificeOthers {
			out = append(out, a.Sacrifices...)
		}
	}
	return out
}
