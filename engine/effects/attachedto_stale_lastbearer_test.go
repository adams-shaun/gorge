package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// staleBearerBoard is a battlefield creature carrying one Equipment, built
// like attachedSelectorBoard (zones set through g.Obj right after AddObject).
func staleBearerBoard(t *testing.T) (*fakeHost, *Ctx, state.ObjID, state.ObjID) {
	t.Helper()
	h := newHost(t, 2)
	hawk := h.g.AddObject(mkCard(t, "Name:Hawk\nManaCost:1 W\nTypes:Creature Bird\nPT:1/1\nOracle:x\n"), 0)
	bone := h.g.AddObject(mkCard(t, "Name:Bonesplitter\nManaCost:1\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	h.g.Obj(hawk.ID).Zone = state.ZBattlefield
	h.g.Obj(bone.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{hawk.ID, bone.ID})
	h.g.Obj(bone.ID).AttachedTo = hawk.ID
	c := &Ctx{Source: hawk.ID, Controller: 0, Targets: []state.Target{{Obj: hawk.ID}}}
	return h, c, hawk.ID, bone.ID
}

func attachedToTargetedIDs(t *testing.T, h *fakeHost, c *Ctx) map[state.ObjID]bool {
	t.Helper()
	ts, ok := definedSpec(h, c, "AttachedTo Targeted.Aura,Equipment")
	if !ok {
		t.Fatal("AttachedTo Targeted.Aura,Equipment classified unknown")
	}
	got := map[state.ObjID]bool{}
	for _, x := range ts {
		got[x.Obj] = true
	}
	return got
}

// TestAttachedToStaleLastBearerOfARecastBearer: Fumble bounces a creature
// carrying an Equipment (the same-resolution read still sees it), the sweep
// detaches it, the creature is recast and a second Fumble targets it. The
// recast creature is a new object (CR 400.7) nothing was ever attached to,
// so the selector must resolve to nothing, while an Equipment that fell off
// in the same resolution (or an earlier sweep of the SAME incarnation) still
// resolves for a queued trigger.
func TestAttachedToStaleLastBearerOfARecastBearer(t *testing.T) {
	h, c, hawk, bone := staleBearerBoard(t)

	// First Fumble: bounce the bearer; before any sweep the selector sees it.
	h.Emit(events.Event{Kind: events.MoveZone, Obj: hawk, From: state.ZBattlefield, To: state.ZHand})
	if got := attachedToTargetedIDs(t, h, c); !got[bone] {
		t.Fatalf("mid-chain read before the sweep: %v, want Bonesplitter %d", got, bone)
	}
	// The sweep publishes the detach; a queued trigger still sees it.
	h.Emit(events.Event{Kind: events.Unattached, Obj: bone, IDs: []state.ObjID{hawk}})
	if o := h.g.Obj(bone); o.AttachedTo != 0 || o.LastBearer != hawk {
		t.Fatalf("precondition failed: Bonesplitter AttachedTo %d LastBearer %d, want 0 and hawk %d", o.AttachedTo, o.LastBearer, hawk)
	}
	if got := attachedToTargetedIDs(t, h, c); !got[bone] {
		t.Fatalf("post-sweep read in the same incarnation: %v, want Bonesplitter %d", got, bone)
	}

	// Recast: the Hawk is a new object on the battlefield.
	before := h.g.Obj(hawk).Incarnation
	h.Emit(events.Event{Kind: events.MoveZone, Obj: hawk, From: state.ZHand, To: state.ZBattlefield})
	if h.g.Obj(hawk).Zone != state.ZBattlefield || h.g.Obj(hawk).Incarnation == before {
		t.Fatalf("precondition failed: Hawk zone %v incarnation %d (was %d), want a re-entered battlefield permanent", h.g.Obj(hawk).Zone, h.g.Obj(hawk).Incarnation, before)
	}
	if o := h.g.Obj(bone); o.AttachedTo != 0 || o.LastBearer != hawk {
		t.Fatalf("precondition failed: Bonesplitter must still carry the stale record, AttachedTo %d LastBearer %d", o.AttachedTo, o.LastBearer)
	}

	// Second Fumble: bounce it again with no equip between.
	h.Emit(events.Event{Kind: events.MoveZone, Obj: hawk, From: state.ZBattlefield, To: state.ZHand})
	if got := attachedToTargetedIDs(t, h, c); len(got) != 0 {
		t.Fatalf("recast bearer: selector resolved %v, want nothing (nothing was attached to the new object)", got)
	}
}

// TestAttachedToFlickeredBearerQueuedDetachIsNoLongerAttached: a bearer that
// left and re-entered before the sweep published the queued detach is a new
// object too; the pending detach's Unattached must not re-arm the record.
func TestAttachedToFlickeredBearerQueuedDetachIsNoLongerAttached(t *testing.T) {
	h, c, hawk, bone := staleBearerBoard(t)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: hawk, From: state.ZBattlefield, To: state.ZExile})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: hawk, From: state.ZExile, To: state.ZBattlefield})
	h.Emit(events.Event{Kind: events.Unattached, Obj: bone, IDs: []state.ObjID{hawk}})
	if o := h.g.Obj(bone); o.AttachedTo != 0 || o.LastBearer != hawk {
		t.Fatalf("precondition failed: AttachedTo %d LastBearer %d", o.AttachedTo, o.LastBearer)
	}
	if got := attachedToTargetedIDs(t, h, c); len(got) != 0 {
		t.Fatalf("flickered bearer: selector resolved %v, want nothing", got)
	}
}
