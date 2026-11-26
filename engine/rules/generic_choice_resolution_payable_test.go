package rules

// A triggered api:GenericChoice whose every UnlessCost$ is unpayable at
// placement goes on the stack unasked (askTriggerModes); effCharm re-reads
// payability at resolution. When a choice became payable in between (a Food
// minted by a trigger resolving first), gorge poses the KModes ask then. The
// dirac kernel must do the same (dirac/modal_unless_resolve_test.go).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
)

func TestNimbleHobbitUnpayableAtPlacementPayableAtResolutionAsks(t *testing.T) {
	t.Parallel()
	e, cfg, hobbit, _ := nimbleHobbitAttacks(t, false)

	// PRECONDITIONS: the trigger is on the stack, placed with no mode ask
	// (nothing payable), and no Food exists yet.
	if !hasTriggerPush(e, hobbit) {
		t.Fatal("precondition: Nimble Hobbit's attack trigger was never pushed")
	}
	if len(e.G.Stack) == 0 {
		t.Fatal("precondition: the attack trigger is not on the stack")
	}
	if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
		t.Fatalf("precondition: pending = %+v, want a priority decision (no placement mode ask)", d)
	}
	for _, ev := range e.L.Events {
		if ev.Kind == events.ModeChosen {
			t.Fatal("precondition: a mode was chosen at placement")
		}
	}

	// A Food appears before the trigger resolves: the sacrifice choice is
	// now payable, so resolution poses the mode ask over that one choice.
	mintToken(t, e, 0, "c_a_food_sac")
	d := passUntilAskKind(t, e, decision.KModes, 80)
	if d.Player != 0 || d.ResumeKind != "modes" || len(d.Options) != 1 {
		t.Fatalf("resolution ask = player %d kind %q with %d options, want seat 0, modes, 1 option", d.Player, d.ResumeKind, len(d.Options))
	}
	submitChoices(t, e, d.Options[0].Index)
	d = passUntilAskKind(t, e, decision.KModes, 80)
	if d.ResumeKind != "unless_pay" {
		t.Fatalf("after the resolution mode ask, next KModes = %q, want unless_pay", d.ResumeKind)
	}
	submitChoices(t, e, d.Options[len(d.Options)-1].Index)
	passUntilStackEmpty(t, e, 80)
	replayCheck(t, e, cfg)
}
