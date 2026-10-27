package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestMoveZoneExileExilerFold pins the MarkExileExiler payload: the fold records
// the exiler on the exiled object (seat 0 included), a move out of exile clears
// it, and an unmarked exile records nothing.
func TestMoveZoneExileExilerFold(t *testing.T) {
	g, id := gameWithOneCard(t)
	l := NewLog(1)
	o := g.Obj(id)
	if o.Zone != state.ZHand || o.HasExiledBy {
		t.Fatalf("precondition: want a hand card with no exiler, got zone %v hasExiledBy %v", o.Zone, o.HasExiledBy)
	}

	Emit(g, l, MarkExileExiler(Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZExile}, 0))
	if o.Zone != state.ZExile {
		t.Fatalf("zone = %v, want exile", o.Zone)
	}
	if !o.HasExiledBy || o.ExiledBy != 0 {
		t.Errorf("exiler = (%v, %d), want (true, 0)", o.HasExiledBy, o.ExiledBy)
	}

	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZExile, To: state.ZHand})
	if o.HasExiledBy || o.ExiledBy != 0 {
		t.Errorf("leaving exile left exiler (%v, %d)", o.HasExiledBy, o.ExiledBy)
	}

	Emit(g, l, MarkExileExiler(Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZExile}, 1))
	if !o.HasExiledBy || o.ExiledBy != 1 {
		t.Errorf("seat 1 exiler = (%v, %d), want (true, 1)", o.HasExiledBy, o.ExiledBy)
	}
	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZExile, To: state.ZHand})

	Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZExile})
	if o.Zone != state.ZExile || o.HasExiledBy {
		t.Errorf("unmarked exile: zone %v hasExiledBy %v, want exile/false", o.Zone, o.HasExiledBy)
	}

	// MarkExileExiler leaves a non-exile move untouched.
	if ev := MarkExileExiler(Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard}, 1); ev.Counter != "" || ev.Amount != 0 {
		t.Errorf("non-exile move was marked: %+v", ev)
	}
}
