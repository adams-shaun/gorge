package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// grantedExaltedTable seats Sublime Archangel (summoning sick, so it cannot
// attack) and a ready 2/2 for seat 0 in its declare-attackers step: the 2/2
// has exalted only through the Archangel's grant.
func grantedExaltedTable(t *testing.T) (e *Engine, bear state.ObjID) {
	t.Helper()
	arch := mshCorpusCardPath(t, "Sublime Archangel", "s/sublime_archangel.txt")
	e = threeSeatEngine(t)
	onBoardCard(t, e, 0, arch)
	bear = onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e, bear
}

// TestGrantedExaltedTriggersPerInstance: CR 702.83a-b, 113.2c. Other creatures
// you control have exalted, and each instance triggers separately: a lone
// attacker with the Archangel's printed exalted and its own granted one gets
// +2/+2.
func TestGrantedExaltedTriggersPerInstance(t *testing.T) {
	t.Parallel()
	e, bear := grantedExaltedTable(t)
	if n := countKeyword(e.Derived(bear).Keywords, "Exalted"); n != 1 {
		t.Fatalf("precondition: the Bear holds %d exalted instances, want the one granted", n)
	}
	e.askAttackers()
	bi := batchAttackerOptionIndex(t, e, bear, 1)
	declareAttackersAt(t, e, [][2]int{{int(bear), bi}})
	if got := exaltedTriggers(e); got != 2 {
		t.Fatalf("seq %d: %d exalted triggers for the lone attacker, want 2 (the Archangel's printed and the Bear's granted)", len(e.L.Events), got)
	}
	flushOrderAsks(t, e)
	if len(e.G.Stack) != 2 {
		t.Fatalf("precondition: %d stack entries after ordering, want the 2 exalted triggers", len(e.G.Stack))
	}
	for len(e.G.Stack) > 0 {
		e.resolveTop()
	}
	if got := e.Power(bear); got != 4 {
		t.Fatalf("seq %d: lone attacker power %d, want 4 (two exalted instances)", len(e.L.Events), got)
	}
}

// TestGrantedExaltedNotForASplitDeclaration: CR 506.5. The grant triggers
// only for a lone attacker, as a printed exalted does.
func TestGrantedExaltedNotForASplitDeclaration(t *testing.T) {
	t.Parallel()
	e, bear := grantedExaltedTable(t)
	fox := onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	e.askAttackers()
	bi := batchAttackerOptionIndex(t, e, bear, 1)
	fi := batchAttackerOptionIndex(t, e, fox, 2)
	declareAttackersAt(t, e, [][2]int{{int(bear), bi}, {int(fox), fi}})
	if got := exaltedTriggers(e); got != 0 {
		t.Fatalf("seq %d: %d exalted triggers for a two-attacker declaration, want 0", len(e.L.Events), got)
	}
}

func countKeyword(ks []string, head string) int {
	n := 0
	for _, k := range ks {
		if k == head {
			n++
		}
	}
	return n
}

// exaltedTriggers counts the exalted triggers the declaration queued: those
// still waiting in the order queue plus those already pushed, printed
// (TriggerPush) or granted (KeywordTriggerPush). The fixtures hold no other
// trigger.
func exaltedTriggers(e *Engine) int {
	n := len(e.pendingTriggers)
	for _, ev := range e.L.Events {
		if ev.Kind == events.KeywordTriggerPush || ev.Kind == events.TriggerPush {
			n++
		}
	}
	return n
}

// TestGrantedExaltedHolderThatDoesNotAttack: the holder need not attack. The
// Archangel attacks alone while the Bear, which holds exalted only through
// the Archangel's grant, stays home: the Archangel's printed instance and the
// Bear's granted one both trigger (CR 702.83a: "a creature you control attacks
// alone").
func TestGrantedExaltedHolderThatDoesNotAttack(t *testing.T) {
	t.Parallel()
	arch := mshCorpusCardPath(t, "Sublime Archangel", "s/sublime_archangel.txt")
	e := threeSeatEngine(t)
	angel := onBoardReady(t, e, 0, "Name:Angel\nManaCost:1 W\nTypes:Creature Angel\nPT:2/2\nOracle:x\n")
	onBoardCard(t, e, 0, arch)
	bear := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	if n := countKeyword(e.Derived(bear).Keywords, "Exalted"); n != 1 {
		t.Fatalf("precondition: the Bear holds %d exalted instances, want the one granted", n)
	}
	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, angel, 1)
	declareAttackersAt(t, e, [][2]int{{int(angel), ai}})
	if got := exaltedTriggers(e); got != 3 {
		t.Fatalf("seq %d: %d exalted triggers, want 3 (the Archangel's printed, the Angel's and the Bear's granted)", len(e.L.Events), got)
	}
}
