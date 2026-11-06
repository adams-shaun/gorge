package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The consumption-only arm's board pre-gate (planBoardConsumesNothing)
// answers, without the census pass, that no alternative the census could
// list for the player consumes its source: a scan of the printed mana and
// ManaReflected abilities on the player's battlefield. Its own contract is
// what the assertions pin -- true on a board whose mana sources only tap,
// false the moment one sacrifices itself, no longer false once it leaves
// the battlefield -- and, through the arm, that a sac-free board's shape
// cast is declined before the census pass runs whatever the floating pool
// holds.
func TestArmBoardConsumesNothingScan(t *testing.T) {
	t.Parallel()
	te, _, _ := newFixtureDeck(t, 9422, ppFiller)
	spell := handCard(te, card(t, acoSpell), 0)
	// Forests are the only sources: their printed abilities only tap, and
	// the {4}{W} printed cost is not pool-coverable, so the arm is entered.
	for i := 0; i < 5; i++ {
		onBoard(t, te, 0, acoForest)
	}
	te.G.Step = state.StepMain1
	ppPriority(t, te)
	if detail := te.paymentPlanCastShapeDetail(0, spell); detail != "shape:target_dependent_cost" {
		t.Fatalf("shape detail %q, want shape:target_dependent_cost", detail)
	}
	te.G.Players[0].Pool[state.MW] = 0
	if !te.planBoardConsumesNothing(0) {
		t.Fatal("a board whose printed mana abilities only tap consumes nothing")
	}
	if te.targetPaidNeedsPlannedPayment(0, spell) {
		t.Fatal("a sac-free board's shape cast is payable only by consuming something")
	}

	// One sacrifice source turns the scan's answer: the census's
	// alternatives could consume it, so the arm is not pre-gated away.
	onBoard(t, te, 0, acoElf)
	if te.planBoardConsumesNothing(0) {
		t.Fatal("a sacrifice mana source on the board consumes something")
	}
}

// acoTapElf's only ability is an ordinary tap ability: a shape the census's
// alternatives never give a consequence to consume with.
const acoTapElf = "Name:Arm Consumption Llanowar\nTypes:Creature Elf\nPower:1\nToughness:1\n" +
	"A:AB$ Mana | Cost$ T | Produced$ G | Amount$ 1 | SpellDescription$ Tap: Add {G}.\nOracle:x\n"

// A tap-only ability source leaves the scan's answer true, exactly like the
// lands' intrinsic taps: the arm is still pre-gated away on its board.
func TestArmBoardScanIgnoresPlainTaps(t *testing.T) {
	t.Parallel()
	te, _, _ := newFixtureDeck(t, 9423, ppFiller)
	for i := 0; i < 5; i++ {
		onBoard(t, te, 0, acoForest)
	}
	onBoard(t, te, 0, acoTapElf)
	te.G.Step = state.StepMain1
	ppPriority(t, te)
	te.G.Players[0].Pool[state.MW] = 0
	if !te.planBoardConsumesNothing(0) {
		t.Fatal("a tap-only ability source consumes nothing")
	}
	if te.targetPaidNeedsPlannedPayment(0, handCard(te, card(t, acoSpell), 0)) {
		t.Fatal("a tap-only board's shape cast is payable only by consuming something")
	}
}
