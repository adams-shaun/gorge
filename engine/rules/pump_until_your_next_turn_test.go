package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPumpUntilYourNextTurnLasts pins CR 611.2b for Pump: a
// Duration$ UntilYourNextTurn grant (Undercity's Throne of the Dead Three
// hexproof) survives the casting turn's cleanup and the opponent's whole
// turn, and ends as the controller's next turn begins. Both the layer-6
// keyword and the layer-7c stat halves carry the lifetime.
func TestPumpUntilYourNextTurnLasts(t *testing.T) {
	t.Parallel()
	spell := card(t, "Name:Ward Off\nManaCost:G\nTypes:Instant\n"+
		"A:SP$ Pump | ValidTgts$ Creature | KW$ Hexproof | NumAtt$ +1 | Duration$ UntilYourNextTurn\nOracle:x\n")
	e := handEngine(t, spell)
	e.G.Players[0].Pool[state.MG] = 1
	creature := onBoard(t, e, 0, "Name:Elf\nManaCost:G\nTypes:Creature Elf\nPT:2/2\nOracle:x\n")
	if e.HasKeyword(creature, "Hexproof") || e.Derived(creature).Power != 2 {
		t.Fatalf("precondition: creature should start 2/2 without hexproof, got %+v", e.Derived(creature))
	}

	e.askPriority(0)
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	idx := -1
	for _, o := range d.Options {
		if o.Obj == creature {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("creature not offered as target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)
	if len(e.G.Stack) != 0 {
		t.Fatalf("spell did not resolve: stack %v", e.G.Stack)
	}

	granted := func(when string) {
		t.Helper()
		if !e.HasKeyword(creature, "Hexproof") || e.Derived(creature).Power != 3 {
			t.Fatalf("%s: want hexproof and power 3, got %+v", when, e.Derived(creature))
		}
	}
	granted("casting turn")
	driveToStepAll(t, e, 2, 1, state.StepMain1)
	granted("opponent's turn main 1")
	driveToStepAll(t, e, 2, 1, state.StepEnd)
	granted("opponent's end step")
	driveToStepAll(t, e, 3, 0, state.StepMain1)
	if e.HasKeyword(creature, "Hexproof") || e.Derived(creature).Power != 2 {
		t.Fatalf("controller's next turn began: want grant gone, got %+v", e.Derived(creature))
	}
}
