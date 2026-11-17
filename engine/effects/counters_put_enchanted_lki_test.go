package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPutCounterEnchantedLKIOnlyOnBattlefield pins the split between
// Defined$ Enchanted's last-known bearer (CR 608.2h) and a counter's
// recipient: after Sadistic Glee's bearer dies and is then exiled, the
// departed Aura still names the card as its last bearer -- the read
// ChangeZone's shuffle uses -- but a +1/+1 counter is not put on the card in
// exile, because the card there is a new object and not a creature
// (CR 400.7). A bearer still on the battlefield takes the counters normally.
func TestPutCounterEnchantedLKIOnlyOnBattlefield(t *testing.T) {
	t.Parallel()
	h := newHost(t, 2)
	giant := h.g.AddObject(mkCard(t, "Name:Giant\nTypes:Creature Giant\nPT:2/2\nOracle:x\n"), 0)
	glee := h.g.AddObject(mkCard(t, "Name:Glee\nTypes:Enchantment Aura\nOracle:x\n"), 0)
	for _, id := range []state.ObjID{giant.ID, glee.ID} {
		h.g.Obj(id).Zone = state.ZBattlefield
	}
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{giant.ID, glee.ID})
	h.Emit(events.Event{Kind: events.Attach, Obj: glee.ID, IDs: []state.ObjID{giant.ID}})
	if o := h.g.Obj(glee.ID); o.AttachedTo != giant.ID {
		t.Fatalf("precondition: Aura AttachedTo = %d, want Giant %d", o.AttachedTo, giant.ID)
	}

	c := &Ctx{Source: glee.ID, Controller: 0}
	pc := sa(t, "DB$ PutCounter | Defined$ Enchanted | CounterType$ P1P1 | CounterNum$ 1")

	// Control: the bearer on the battlefield is the counter's recipient.
	if got := putCounterDefined(h, c, pc); len(got) != 1 || got[0].Obj != giant.ID {
		t.Fatalf("attached battlefield bearer = %v, want Giant %d", got, giant.ID)
	}
	Resolve(h, c, pc)
	if n := counterChangeCount(h, giant.ID); n != 1 {
		t.Fatalf("attached battlefield bearer: %d CounterChange events, want 1", n)
	}

	// The Giant dies; the Aura leaves as a state-based action while the
	// bearer-departure detach is still queued; Withered Wretch then exiles
	// the Giant from the graveyard.
	h.log = h.log[:0]
	h.Emit(events.Event{Kind: events.MoveZone, Obj: giant.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: glee.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	h.Emit(events.Event{Kind: events.Unattached, Obj: glee.ID, IDs: []state.ObjID{giant.ID}})
	h.Emit(events.Event{Kind: events.MoveZone, Obj: giant.ID, From: state.ZGraveyard, To: state.ZExile})
	if o := h.g.Obj(glee.ID); o.Zone != state.ZGraveyard || o.LastBearer != giant.ID {
		t.Fatalf("precondition: departed Aura zone %s LastBearer %d, want graveyard / Giant %d", o.Zone, o.LastBearer, giant.ID)
	}
	if o := h.g.Obj(giant.ID); o.Zone != state.ZExile {
		t.Fatalf("precondition: Giant zone %s, want exile", o.Zone)
	}

	// The selector still names the last-known bearer (the shuffle path).
	ts, ok := definedSpec(h, c, "Enchanted")
	if !ok || len(ts) != 1 || ts[0].Obj != giant.ID {
		t.Fatalf("departed Enchanted = %v, %v; want the last bearer Giant %d", ts, ok, giant.ID)
	}
	// The counter is not put on the card in exile.
	h.log = h.log[:0]
	Resolve(h, c, pc)
	if n := counterChangeCount(h, giant.ID); n != 0 {
		t.Fatalf("departed Enchanted: %d CounterChange events on the exiled card, want 0", n)
	}
}
