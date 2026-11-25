package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTeferisTimeTwistReturnsWithCounter pins the real corpus card's
// end-of-turn return rider: the spell exiles a permanent and creates a
// delayed trigger whose Effect registers "If it enters as a creature, it
// enters with an additional +1/+1 counter on it" (CR 614.1c / 614.12). The
// exiled 2/2 returns at the beginning of the end step with exactly one +1/+1
// counter.
func TestTeferisTimeTwistReturnsWithCounter(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	e, cfg := ateotEngine(t, reg, "Teferi's Time Twist")

	twist := ateotFind(t, e, "Teferi's Time Twist", 0)
	if o := e.G.Obj(twist); o.Zone != state.ZHand {
		ateotTo(t, e, twist, o.Zone, state.ZHand)
	}
	bear := ateotFind(t, e, "Grizzly Bears", 0)
	if o := e.G.Obj(bear); o.Zone != state.ZBattlefield {
		ateotTo(t, e, bear, o.Zone, state.ZBattlefield)
	}

	// Preconditions the assertions depend on: the spell is in hand, the
	// target is a 2/2 on the battlefield, and it has no +1/+1 counter yet.
	if o := e.G.Obj(twist); o == nil || o.Zone != state.ZHand {
		t.Fatalf("precondition: Teferi's Time Twist is %+v, want it in hand", o)
	}
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZBattlefield || o.Face() == nil ||
		o.Face().Power() != 2 || o.Face().Toughness() != 2 {
		t.Fatalf("precondition: target is %+v, want a 2/2 on the battlefield", o)
	}
	if got := e.G.Obj(bear).Counter("P1P1"); got != 0 {
		t.Fatalf("precondition: target starts with %d +1/+1 counters, want 0", got)
	}

	addMana(t, e, 0, "UU")
	castNamed(t, e, "Teferi's Time Twist")
	passUntilKind(t, e, decision.KTarget, 40)
	submitTarget(t, e, bear)
	passUntilStackEmpty(t, e, 40)

	// The spell exiled the target and created the delayed return.
	if o := e.G.Obj(bear); o == nil || o.Zone != state.ZExile {
		t.Fatalf("target zone after the spell = %v, want exile", zoneName(e.G.Obj(bear)))
	}
	if len(e.G.Delayed) != 1 {
		t.Fatalf("delayed registrations = %d, want 1", len(e.G.Delayed))
	}

	driveToEndStep(t, e)
	for e.putTriggersOnStack() {
		answerTriggerOrders(t, e)
		passUntilStackEmpty(t, e, 40)
	}
	passUntilStackEmpty(t, e, 40)

	o := e.G.Obj(bear)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("returned creature zone = %v, want battlefield", zoneName(o))
	}
	if got := o.Counter("P1P1"); got != 1 {
		t.Fatalf("returned creature has %d +1/+1 counters, want exactly 1", got)
	}
	replayCheck(t, e, cfg)
}
