package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 400.7: an object that moves from one zone to another becomes a new
// object with no memory of its previous existence. A counter placed on the
// card while it sat in a public zone off the battlefield -- Torture's
// {1}{B}: "put a -1/-1 counter on enchanted creature" resolving after the
// creature died -- does not follow it when the card re-enters the
// battlefield: the entering permanent is a new object and the wayward
// counter is dropped at the entry. A spell resolving onto the battlefield
// is the stack arm (CR 400.7a-b) and keeps its stack counters.
func TestEntryDropsCountersTheCardCarriedInItsPublicZone(t *testing.T) {
	t.Parallel()
	card := corpusAlternativeCard(t, "Sanguine Syphoner")
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, card)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: Sanguine Syphoner is not on the battlefield")
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	if got := o.Counter("P1P1"); got != 2 {
		t.Fatalf("precondition: battlefield P1P1 counters = %d, want 2", got)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZGraveyard})
	o = e.G.Obj(id)
	if got := o.Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 counters after dying = %d, want 0 (CR 400.7 departure)", got)
	}
	// The wayward counter: a resolving ability whose defined creature has
	// already left the battlefield puts it on the graveyard card.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "M1M1", Amount: 1})
	if got := o.Counter("M1M1"); got != 1 {
		t.Fatalf("precondition: wayward M1M1 counter on the graveyard card = %d, want 1", got)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZGraveyard, To: state.ZBattlefield})
	o = e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("re-entry zone = %s, want battlefield", o.Zone)
	}
	if got := o.Counter("M1M1"); got != 0 {
		t.Fatalf("wayward M1M1 counter after re-entry = %d, want 0 (CR 400.7 entry)", got)
	}
	if got := o.Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 counters after re-entry = %d, want 0", got)
	}
}

// The stash may-play marker (Tinybones, Bauble Burglar's "with a stash
// counter on it") is the one keeper beyond the CountersRemain exception: it
// grants the may-play from exile and is kept on the card played from exile,
// which the kernel mirrors in its projection, so the CR 400.7 entry clear
// keeps it while dropping every other counter the card carried.
func TestEntryKeepsTheStashMayPlayMarker(t *testing.T) {
	t.Parallel()
	card := corpusAlternativeCard(t, "Sanguine Syphoner")
	e := combatEngine(t)
	id := onBoardCard(t, e, 0, card)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZBattlefield, To: state.ZExile})
	o := e.G.Obj(id)
	// Tinybones's trigger exiles the card with a stash counter beside the
	// wayward counter the entry clear is about to drop.
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "STASH", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 1})
	if o.Zone != state.ZExile {
		t.Fatalf("precondition: the card is in %s, want exile", o.Zone)
	}
	if got := o.Counter("STASH"); got != 1 {
		t.Fatalf("precondition: stash markers on the exiled card = %d, want 1", got)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZExile, To: state.ZBattlefield})
	o = e.G.Obj(id)
	if o.Zone != state.ZBattlefield {
		t.Fatalf("played-from-exile zone = %s, want battlefield", o.Zone)
	}
	if got := o.Counter("STASH"); got != 1 {
		t.Fatalf("stash markers after playing from exile = %d, want 1 (gorge keeps the may-play marker)", got)
	}
	if got := o.Counter("P1P1"); got != 0 {
		t.Fatalf("P1P1 counters after playing from exile = %d, want 0 (CR 400.7 entry)", got)
	}
}
