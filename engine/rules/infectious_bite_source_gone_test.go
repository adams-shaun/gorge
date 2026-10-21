package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestInfectiousBiteSourceGone pins CR 608.2b for a DamageSource$ ParentTarget
// whose parent target is illegal by resolution: Infectious Bite's first target
// (the creature you control) is gone, so that creature deals no damage to the
// second target, while the spell's independent poison rider still resolves.
func TestInfectiousBiteSourceGone(t *testing.T) {
	e, cfg, mine, theirs := cr601Board(t, 90210,
		map[string]state.Zone{"Infectious Bite": state.ZHand, "Grizzly Bears": state.ZBattlefield},
		map[string]state.Zone{"Serra Angel": state.ZBattlefield})
	bears, angel, spell := mine["Grizzly Bears"], theirs["Serra Angel"], mine["Infectious Bite"]
	addMana(t, e, 0, "1G")
	cr601Cast(t, e, spell, "")
	answerTargetAsk(t, e, []state.ObjID{bears})
	answerCastSubObj(t, e, angel)
	// PRECONDITION: the named source is on the battlefield and the victim is
	// undamaged, so the compared values differ from the expected result.
	if o := e.G.Obj(bears); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears zone = %v, want battlefield", o.Zone)
	}
	if o := e.G.Obj(angel); o == nil || o.Damage != 0 {
		t.Fatalf("precondition: Serra Angel damage = %d, want 0", o.Damage)
	}
	// The first target is destroyed in response: it is no longer a legal
	// target, so its information is not used (CR 608.2b).
	e.emit(events.Event{Kind: events.MoveZone, Obj: bears, From: state.ZBattlefield, To: state.ZGraveyard})
	mark := len(e.L.Events)
	cr601ResolveQuietly(t, e)
	if got := cr18cDamageTo(e, mark, angel); len(got) != 0 {
		t.Fatalf("Serra Angel took %+v, want no damage: the gone first target is not a damage source", got)
	}
	if got := e.G.Players[1].Counter("POISON"); got != 1 {
		t.Fatalf("seat 1 poison = %d, want 1: the poison rider resolves independently", got)
	}
	replayCheck(t, e, cfg)
}
