package rules

// The overlap re-base path for the TRIGGER form of api:Clone GainThisAbility$:
// when a clone unit expires while a
// surviving unit whose root is a TRIGGER remains on the same permanent,
// settleExpiredClones must re-emit the survivor as Counter
// "gain-this-trigger" with its one-based trigger index, not the ability form
// (which would append a wrong/none ability). No corpus carrier stacks two
// clone units on one permanent, so this registers the two markers directly --
// the machinery, not a card route.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// triggerCloneFixture is a permanent whose printed face carries one trigger
// (the recurring-copy root) so the re-base's fold has a trigger to append.
const triggerCloneFixture = "Name:Fixture Recurring Copy\nManaCost:2\nTypes:Artifact\n" +
	"T:Mode$ Phase | Phase$ Upkeep | Execute$ CopyBody\n" +
	"SVar:CopyBody:DB$ Clone | GainThisAbility$ True\n" +
	"Oracle:x\n"

func TestSettleExpiredCloneRebasesOntoTriggerFormSurvivor(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 145, triggerCloneFixture, cloneOxSrc)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZBattlefield})
	ox := moveSeeded(t, e, 0, cloneOxSrc, state.ZBattlefield)
	obj := e.G.Obj(id)
	if obj == nil || obj.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the fixture permanent must be on the battlefield, got %+v", obj)
	}
	if src := e.G.Obj(ox); src == nil || src.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the copy source must be on the battlefield, got %+v", src)
	}
	if f := obj.Face(); f == nil || len(f.Triggers) != 1 || f.Triggers[0].Effect == nil {
		t.Fatalf("precondition: the fixture must carry one compiled trigger, got %+v", obj.Face())
	}
	// A survivor layer whose root is the trigger (applied first, the older
	// timestamp), then a newer plain copy of the Ox that is the one expiring.
	// The survivor's locked face carries the trigger (CR 707.2b/c), so the
	// re-base needs no source read.
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: id, IDs: []state.ObjID{ox},
		Counter: "gain-this-trigger", Amount: 1})
	survivor := ContinuousEffect{Layer: LCopy, CloneTarget: id, CloneSource: ox,
		CloneGainThisAbility: true, CloneTriggerIndex: 1, Timestamp: 1, Controller: 0,
		CloneSeq: e.G.Obj(id).CloneSeq}
	e.emit(events.Event{Kind: events.ClonePermanent, Obj: id, IDs: []state.ObjID{ox}})
	if f := e.G.Obj(id).Face(); f == nil || len(f.Triggers) != 0 {
		t.Fatalf("precondition: the expiring plain copy must hide the trigger, got %+v", e.G.Obj(id).Face())
	}
	e.continuous = append(e.continuous, survivor)

	e.settleExpiredClones([]cloneExpiry{{Target: id}})

	// The fold restored the survivor's locked face and the trigger rode it.
	if f := e.G.Obj(id).Face(); f == nil || len(f.Triggers) != 1 {
		t.Fatalf("re-based copy must carry the survivor's one trigger, got %+v", e.G.Obj(id).Face())
	}
}
