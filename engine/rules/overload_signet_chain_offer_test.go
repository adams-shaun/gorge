package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOverloadOfferCountsSignetOutputTowardColourless pins CR 601.2f-h and
// 605.3b: Eldritch Immunity's overload cost {4}{C} is payable from Forest, two
// Swamps, a storage land that taps for {C} and a Signet (Forest pays the
// Signet's {1}, its {R}{W} and the Swamps pay the generic part, the land pays
// {C}), so the overloaded cast is offered on the autopay surface even with no
// creature to target. dirac prices the cast from its untapped sources and
// offered it where gorge's potential walk priced nothing.
func TestOverloadOfferCountsSignetOutputTowardColourless(t *testing.T) {
	t.Parallel()
	const forest = "Name:Forest\nTypes:Basic Land Forest\nOracle:x\n"
	const swamp = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"
	const net = "Name:Net\nTypes:Land\n" +
		"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Add C.\n" +
		"A:AB$ PutCounter | Cost$ 1 T | CounterType$ STORAGE | CounterNum$ 1 | SpellDescription$ Store.\n" +
		"A:AB$ Mana | Cost$ T SubCounter<X/STORAGE> | Produced$ C | Amount$ X | SpellDescription$ Spend.\n" +
		"SVar:X:Count$xPaid\nOracle:x\n"
	const signet = "Name:Boros Signet\nManaCost:2\nTypes:Artifact\n" +
		"A:AB$ Mana | Cost$ 1 T | Produced$ R W | SpellDescription$ Add RW.\nOracle:x\n"
	immunity, ok := testutil.CorpusRegistry(t).Lookup("Eldritch Immunity")
	if !ok {
		t.Fatal("registry lacks Eldritch Immunity")
	}
	overloaded := func(board ...string) bool {
		e := mayPlayBase(t)
		spell := handCard(e, immunity, 0)
		for _, c := range board {
			onBoardGrant(t, e, 0, c)
		}
		putCountersOn(t, e, 0, "Net", "STORAGE", 1)
		e.priorityRound()
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind == "cast" && pp.Action.Obj == spell && pp.Action.Mode == "overloaded" && pp.Reason == "" {
				return true
			}
		}
		return false
	}
	if !overloaded(forest, swamp, swamp, swamp, net) {
		t.Fatal("precondition: four basics and the storage land pay {4}{C}")
	}
	if overloaded(forest, swamp, swamp, net) {
		t.Fatal("precondition: four mana sources cannot pay {4}{C}")
	}
	if !overloaded(forest, swamp, swamp, net, signet) {
		t.Fatal("overloaded cast withheld though Forest->Signet makes the fifth mana")
	}
}
