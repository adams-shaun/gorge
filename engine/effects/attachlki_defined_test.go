package effects

// This file pins definedSpec's definedSpecEquipped case ("Defined$
// Enchanted|Equipped|AttachedTo") at its two boundary reads:
//
//   - CR 608.2h last known information: a source that LEFT the battlefield
//     as a cost ("{5}, Sacrifice this Aura: Exile enchanted creature") still
//     names its last bearer through LastBearer, which the events.Apply move
//     fold preserved when it cleared the live AttachedTo link.
//   - CR 400.7: the fallback is only for a source OFF the battlefield. A
//     source still on the battlefield that is merely unattached -- a bearer
//     that died, an Equipment that left and returned unattached -- is a new
//     object with no "enchanted/equipped creature", so it must resolve to
//     the empty set, never to a stale LastBearer.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

func TestDefinedSpecEquippedLastBearerOnlyOffBattlefield(t *testing.T) {
	h := newHost(t, 2)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	equip := h.g.AddObject(mkCard(t, "Name:Splitter\nManaCost:1\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	h.g.Obj(bear.ID).Zone = state.ZBattlefield
	h.g.Obj(equip.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{bear.ID, equip.ID})
	c := &Ctx{Source: equip.ID, Controller: 0}

	// Live attach through the real fold.
	h.Emit(events.Event{Kind: events.Attach, Obj: equip.ID, IDs: []state.ObjID{bear.ID}})
	if o := h.g.Obj(equip.ID); o.AttachedTo != bear.ID {
		t.Fatalf("precondition: live attach AttachedTo = %d, want %d", o.AttachedTo, bear.ID)
	}
	// Control: a live-attached battlefield source resolves its bearer.
	ts, ok := definedSpec(h, c, "Equipped")
	if !ok || len(ts) != 1 || ts[0].Obj != bear.ID {
		t.Fatalf("live-attached Equipped = %v, %v; want the bearer %d", ts, ok, bear.ID)
	}

	// The source leaves the battlefield (sacrificed as a cost): the fold
	// clears AttachedTo and preserves the bearer as LastBearer, so the LKI
	// read still resolves it.
	h.Emit(events.Event{Kind: events.MoveZone, Obj: equip.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := h.g.Obj(equip.ID); o.Zone != state.ZGraveyard || o.AttachedTo != 0 || o.LastBearer != bear.ID {
		t.Fatalf("precondition: off-battlefield source %+v, want graveyard / AttachedTo 0 / LastBearer %d", o, bear.ID)
	}
	ts, ok = definedSpec(h, c, "Equipped")
	if !ok || len(ts) != 1 || ts[0].Obj != bear.ID {
		t.Fatalf("off-battlefield Equipped = %v, %v; want the last bearer %d", ts, ok, bear.ID)
	}

	// CR 400.7: the source returns to the battlefield as a NEW object, so the
	// fold drops the stale LastBearer and the now-unattached Equipment
	// resolves to nothing.
	h.Emit(events.Event{Kind: events.MoveZone, Obj: equip.ID, From: state.ZGraveyard, To: state.ZBattlefield})
	if o := h.g.Obj(equip.ID); o.Zone != state.ZBattlefield || o.AttachedTo != 0 || o.LastBearer != 0 {
		t.Fatalf("precondition: returned source %+v, want battlefield / AttachedTo 0 / LastBearer 0", o)
	}
	ts, ok = definedSpec(h, c, "Equipped")
	if !ok || len(ts) != 0 {
		t.Fatalf("returned unattached Equipped = %v, %v; want the empty set", ts, ok)
	}

	// The bearer dies while the Equipment stays on the battlefield: the SBA
	// Unattached fold records the dead bearer, but the source is still a
	// battlefield permanent and must not resolve it.
	h.Emit(events.Event{Kind: events.Attach, Obj: equip.ID, IDs: []state.ObjID{bear.ID}})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: bear.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	h.Emit(events.Event{Kind: events.Unattached, Obj: equip.ID, IDs: []state.ObjID{bear.ID}})
	if o := h.g.Obj(equip.ID); o.Zone != state.ZBattlefield || o.AttachedTo != 0 || o.LastBearer != bear.ID {
		t.Fatalf("precondition: bearer-died source %+v, want battlefield / AttachedTo 0 / LastBearer %d", o, bear.ID)
	}
	ts, ok = definedSpec(h, c, "Equipped")
	if !ok || len(ts) != 0 {
		t.Fatalf("bearer-died battlefield Equipped = %v, %v; want the empty set", ts, ok)
	}
}

// An Equipment detached by a state-based action while it stayed on the
// battlefield (its bearer stopped being a creature) was attached to nothing when
// it later left, so "equipped creature" names nothing (CR 608.2h) rather than
// the bearer the earlier detach recorded.
func TestDefinedSpecEquippedNotStaleAfterDetachThenLeave(t *testing.T) {
	h := newHost(t, 2)
	bear := h.g.AddObject(mkCard(t, "Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	equip := h.g.AddObject(mkCard(t, "Name:Splitter\nManaCost:1\nTypes:Artifact Equipment\nOracle:x\n"), 0)
	h.g.Obj(bear.ID).Zone = state.ZBattlefield
	h.g.Obj(equip.ID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{bear.ID, equip.ID})
	c := &Ctx{Source: equip.ID, Controller: 0}

	h.Emit(events.Event{Kind: events.Attach, Obj: equip.ID, IDs: []state.ObjID{bear.ID}})
	h.Emit(events.Event{Kind: events.Unattached, Obj: equip.ID, IDs: []state.ObjID{bear.ID}})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: equip.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	if ts, ok := definedSpec(h, c, "Equipped"); !ok || len(ts) != 0 {
		t.Fatalf("Equipped after leaving unattached = %v, %v; want nothing", ts, ok)
	}
}
