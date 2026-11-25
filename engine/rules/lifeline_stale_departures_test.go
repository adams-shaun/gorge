package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A trigger window that ends by a direct restore of triggerBefore (the
// legend-rule answer here) must retire its departure list with it: the unchosen
// duplicate can return to the battlefield, and a later death's intervening-if
// must count it as the "another creature" it then is.
func TestLifelineStaleLegendDepartureDoesNotHideAReturnedCreature(t *testing.T) {
	t.Parallel()
	e, id1, id2 := legendPairGame(t, "Stale Twin")
	lifeline := onBoard(t, e, 0, lbLifeline)
	bear := onBoard(t, e, 0, lbBear)
	e.checkStateBased()
	d := legendPending(t, e, 0, id1, id2)
	submitKeep(t, e, d, 1)
	if o := e.G.Obj(id1); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: unchosen duplicate in %v, want graveyard", o.Zone)
	}
	if e.triggerBefore != nil {
		t.Fatalf("precondition: the legend answer left a window open")
	}
	// Fixture surgery with no event, so no window closes and clears the list:
	// the kept duplicate is set aside and the unchosen one comes back. Then
	// the bear dies: id1 is the only creature left on the battlefield, the
	// "another creature" of Lifeline's condition.
	e.G.Obj(id2).Zone = state.ZExile
	bf := []state.ObjID{}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if id != id2 {
			bf = append(bf, id)
		}
	}
	e.G.SetZone(state.ZBattlefield, 0, bf)
	e.emit(events.Event{Kind: events.MoveZone, Obj: id1, From: state.ZGraveyard, To: state.ZBattlefield})
	if o := e.G.Obj(id1); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: returned duplicate in %v, want battlefield", o.Zone)
	}
	e.pendingTriggers = nil
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZBattlefield, To: state.ZGraveyard})
	if o := e.G.Obj(bear); o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: bear in %v, want graveyard", o.Zone)
	}
	if n := lbQueuedFor(e, lifeline); n != 1 {
		t.Fatalf("Lifeline queued %d triggers with a creature on the battlefield, want 1", n)
	}
}
