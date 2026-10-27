package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGracefulTakedownBothPumpPicksDealDamage casts Graceful Takedown with an
// enchanted creature as the root Pump's pick, a DIFFERENT creature as the
// sub-link DBPumpBis pick, and an opposing creature as the damage target.
// DBPumpBis's RememberObjects$ ThisTargetedCard must remember its OWN pick
// (not re-remember the root's), so DBEachDamage's Remembered damagers are both
// creatures and the target takes power(A)+power(B). The target's toughness
// sits between either single power and the sum, so only the sum kills it.
func TestGracefulTakedownBothPumpPicksDealDamage(t *testing.T) {
	reg := searchTestRegistry(t)
	e, cfg := corpusEngineCfg(t, reg,
		[]*cards.Card{lookup(t, reg, "Graceful Takedown"), lookup(t, reg, "Grizzly Bears"), lookup(t, reg, "Hill Giant"), lookup(t, reg, "Pacifism")},
		[]*cards.Card{lookup(t, reg, "Hollowhenge Beast")})
	bears := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	giant := moveByName(t, e, 0, "Hill Giant", state.ZBattlefield)
	aura := moveByName(t, e, 0, "Pacifism", state.ZBattlefield)
	beast := moveByName(t, e, 1, "Hollowhenge Beast", state.ZBattlefield)
	e.emit(events.Event{Kind: events.Attach, Obj: aura, IDs: []state.ObjID{bears}})
	moveByName(t, e, 0, "Graceful Takedown", state.ZHand)
	addMana(t, e, 0, "1G")

	// Preconditions: the Aura is on the Bears only, the powers differ, and
	// the target's toughness exceeds either power (and twice the Bears') but
	// not their sum, so only both damagers together kill it.
	pa, pb := e.Power(bears), e.Power(giant)
	tough := e.Toughness(beast)
	if e.G.Obj(aura).AttachedTo != bears || e.G.Obj(giant).AttachedTo != 0 || pa == pb {
		t.Fatalf("precondition: aura on %d (bears %d), powers %d/%d", e.G.Obj(aura).AttachedTo, bears, pa, pb)
	}
	if tough <= pa || tough <= pb || tough <= 2*pa || tough > pa+pb {
		t.Fatalf("precondition: toughness %d vs powers %d/%d", tough, pa, pb)
	}

	castFirst(t, e, "cast")
	// Root pick (the enchanted creature), then DBPumpBis's own pick, then
	// DBEachDamage's target.
	for _, pick := range []state.ObjID{bears, giant, beast} {
		targetObject(t, e, pick)
	}
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(beast); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("target took less than power(%d)+power(%d)=%d: %+v", pa, pb, pa+pb, o)
	}
	replayCheck(t, e, cfg)
}
