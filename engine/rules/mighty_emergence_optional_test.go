package rules

// Mighty Emergence: "Whenever a creature you control with power 5 or greater
// enters, you may put two +1/+1 counters on it." CR 603.5: the controller
// chooses at resolution whether the optional effect happens, so an unanswered
// "you may" declines and the creature enters with no counters.
//
// This test drives the REAL corpus card rather than an inline corrected
// script. The defect is in the card data (the Forge script carried no
// OptionalDecider$), so a fixture that spelled the parameter into its own
// trigger line would pass while the shipped corpus stayed mandatory; that is
// exactly how the Rage Forger fix never landed. Loading the corpus card makes
// the assertion fail until the script itself is corrected.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mightyEmergenceOptionalAsk puts the real Mighty Emergence on seat 0's
// battlefield, moves the real Woolly Thoctar (5/4) onto the battlefield, and
// returns the engine plus the resolution-time KTriggerOptional ask. The
// returned creature is the permanent the trigger's Defined$ names.
func mightyEmergenceOptionalAsk(t *testing.T) (*Engine, *decision.Decision, state.ObjID) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	mighty, ok := reg.Lookup("Mighty Emergence")
	if !ok {
		t.Fatal("corpus has no Mighty Emergence")
	}
	thoctar, ok := reg.Lookup("Woolly Thoctar")
	if !ok {
		t.Fatal("corpus has no Woolly Thoctar")
	}

	e := layerEngine(t)
	source := onBoardCard(t, e, 0, mighty)
	// PRECONDITION: the trigger's source is a battlefield permanent, so the
	// ChangesZone line's TriggerZones$ Battlefield gate can see it.
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Mighty Emergence = %+v, want a battlefield permanent", o)
	}

	// Park the real 5/4 in seat 0's hand, then let it enter with a logged
	// MoveZone so the ChangesZone trigger fires exactly as it does in a game.
	o := e.G.AddObject(thoctar, 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})

	// PRECONDITION: the entering creature matches Creature.powerGE5+YouCtrl,
	// so a missing trigger is not a filter miss.
	if got := e.G.Obj(o.ID).Zone; got != state.ZBattlefield {
		t.Fatalf("precondition: Woolly Thoctar zone = %v, want the battlefield", got)
	}
	if got := e.Power(o.ID); got < 5 {
		t.Fatalf("precondition: Woolly Thoctar power = %d, want >= 5", got)
	}
	if got := len(e.pendingTriggers); got != 1 {
		t.Fatalf("Mighty Emergence queued %d triggers for one entering 5/4, want 1", got)
	}

	e.putTriggersOnStack()
	if got := len(e.G.Stack); got != 1 {
		t.Fatalf("trigger placement left %d stack item(s), want 1", got)
	}
	// CR 603.5: the optional trigger goes on the stack regardless and asks as
	// it resolves, so resolving it is where the may appears.
	e.resolveTop()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTriggerOptional {
		t.Fatalf("the trigger resolved without posing KTriggerOptional: Woolly Thoctar P1P1 = %d, decision = %+v",
			e.G.Obj(o.ID).Counter("P1P1"), d)
	}
	if d.Player != 0 || len(d.Options) != 2 {
		t.Fatalf("optional election = %+v, want seat 0's yes/no", d)
	}
	return e, d, o.ID
}

// drainMightyStack passes priority until the trigger has left the stack.
func drainMightyStack(t *testing.T, e *Engine) {
	t.Helper()
	for i := 0; i < 20 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("unexpected decision after the election: %+v", d)
		}
		passOnce(t, e)
	}
	if n := len(e.G.Stack); n != 0 {
		t.Fatalf("stack still holds %d item(s)", n)
	}
}

func TestMightyEmergenceTriggerIsOptional(t *testing.T) {
	t.Parallel()
	t.Run("no", func(t *testing.T) {
		t.Parallel()
		e, d, thoctar := mightyEmergenceOptionalAsk(t)
		submitChoices(t, e, d.Options[1].Index)
		drainMightyStack(t, e)
		if got := e.G.Obj(thoctar).Counter("P1P1"); got != 0 {
			t.Fatalf("declining left %d +1/+1 counter(s), want 0", got)
		}
	})
	t.Run("yes", func(t *testing.T) {
		t.Parallel()
		e, d, thoctar := mightyEmergenceOptionalAsk(t)
		submitChoices(t, e, d.Options[0].Index)
		drainMightyStack(t, e)
		if got := e.G.Obj(thoctar).Counter("P1P1"); got != 2 {
			t.Fatalf("accepting put %d +1/+1 counter(s), want 2", got)
		}
	})
}
