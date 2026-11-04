package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedAttackersDeclaredBatchFiresOnceOnSplitAttack pins CR 508.1 for a
// GRANTED "whenever you attack" trigger (a static's AddTrigger$, the shape a
// station-gated Spacecraft uses): declaring attackers at two defenders emits
// two DeclareAttackers events but is one turn-based action, so the granted
// batch trigger is queued once. The printed-trigger walk latched this already;
// the granted walk did not and queued one per defender.
func TestGrantedAttackersDeclaredBatchFiresOnceOnSplitAttack(t *testing.T) {
	t.Parallel()
	e := threeSeatEngine(t)
	onBoard(t, e, 0, "Name:Grantor\nManaCost:2\nTypes:Artifact\n"+
		"S:Mode$ Continuous | AffectedDefined$ Self | Affected$ Card.Self | AddTrigger$ AtkTrig | Description$ x\n"+
		"SVar:AtkTrig:Mode$ AttackersDeclared | AttackingPlayer$ You | Execute$ Draw1 | TriggerZones$ Battlefield | TriggerDescription$ x\n"+
		"SVar:Draw1:DB$ Draw | NumCards$ 1\nOracle:x\n")
	a := onBoardReady(t, e, 0, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	b := onBoardReady(t, e, 0, "Name:Fox\nManaCost:1 W\nTypes:Creature Fox\nPT:2/2\nOracle:x\n")
	e.G.Active = 0
	e.G.Step = state.StepDeclareAttackers

	e.askAttackers()
	ai := batchAttackerOptionIndex(t, e, a, 1)
	bi := batchAttackerOptionIndex(t, e, b, 2)
	declareAttackersAt(t, e, [][2]int{{int(a), ai}, {int(b), bi}})

	groups := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.DeclareAttackers {
			groups++
		}
	}
	if groups != 2 {
		t.Fatalf("precondition failed: %d DeclareAttackers events, want 2 (one per defender)", groups)
	}
	// Two queued instances would pose an ordering prompt (CR 603.3b) before
	// either is pushed, which is the over-fire's visible shape.
	if d := e.Pending(); d != nil && d.Kind == decision.KTriggerOrder {
		t.Fatalf("one granted batch trigger posed a trigger-order prompt over %d options: it queued once per defender", len(d.Options))
	}
	// A granted line rides GrantTriggerPush (not TriggerPush).
	if got := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.GrantTriggerPush }); got != 1 {
		t.Fatalf("granted batch trigger pushed %d times for one split declaration, want 1", got)
	}
}
