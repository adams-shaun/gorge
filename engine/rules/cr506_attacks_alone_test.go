package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// cr506AloneSeat builds a three-seat table in seat 0's declare-attackers
// step: Akrasan Squire (K:Exalted, summoning sick so it cannot attack) plus
// two ready 2/2s, Bear and Fox.
func cr506AloneSeat(t *testing.T) (e *Engine, bear, fox state.ObjID) {
	t.Helper()
	squire := mshCorpusCardPath(t, "Akrasan Squire", "a/akrasan_squire.txt")
	e = threeSeatEngine(t)
	onBoardCard(t, e, 0, squire)
	bear = onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	fox = onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers
	return e, bear, fox
}

// TestCR506_5AttacksAloneSpansTheWholeDeclaration: CR 506.5 -- a creature
// attacks alone only if it is the ONLY creature declared as an attacker
// during the declare attackers step. Bear at seat 1 and Fox at seat 2 is one
// declaration of two attackers (two DeclareAttackers events, one per
// defender), so Exalted must not trigger for either.
func TestCR506_5AttacksAloneSpansTheWholeDeclaration(t *testing.T) {
	t.Parallel()
	e, bear, fox := cr506AloneSeat(t)
	e.askAttackers()
	bi := batchAttackerOptionIndex(t, e, bear, 1)
	fi := batchAttackerOptionIndex(t, e, fox, 2)
	declareAttackersAt(t, e, [][2]int{{int(bear), bi}, {int(fox), fi}})
	// Two queued Exalted triggers wait for a trigger_order answer instead of
	// pushing, so count queued and pushed together.
	if got := batchTriggerPushes(e) + len(e.pendingTriggers); got != 0 {
		t.Fatalf("CR 506.5 seq %d: exalted triggered %d times for a two-attacker split declaration, want 0", len(e.L.Events), got)
	}
	if e.Power(bear) != 2 || e.Power(fox) != 2 {
		t.Fatalf("CR 506.5 seq %d: power Bear %d Fox %d, want 2 and 2", len(e.L.Events), e.Power(bear), e.Power(fox))
	}
}

// TestCR506_5LoneAttackerStillExalted is the guard: one attacker declared
// alone still triggers Exalted once and gets +1/+1.
func TestCR506_5LoneAttackerStillExalted(t *testing.T) {
	t.Parallel()
	e, bear, _ := cr506AloneSeat(t)
	e.askAttackers()
	bi := batchAttackerOptionIndex(t, e, bear, 2)
	declareAttackersAt(t, e, [][2]int{{int(bear), bi}})
	if got := batchTriggerPushes(e); got != 1 {
		t.Fatalf("CR 506.5 seq %d: exalted pushed %d triggers for a lone attacker, want 1", len(e.L.Events), got)
	}
	e.resolveTop()
	if got := e.Power(bear); got != 3 {
		t.Fatalf("CR 506.5 seq %d: lone attacker power %d, want 3", len(e.L.Events), got)
	}
}
