package rules

// A ChangeZoneAll sweep is ONE zone-change event (CR 603.2c), whichever zones
// it bridges: effChangeZoneAll brackets every sweep in its zone batch, so a
// "one or more" line over the swept set queues once for the whole sweep, not
// once per moved card. Before this bracket was unconditional, a
// graveyard->exile/hand/library sweep (Tormod's Crypt beside Ketramose or
// Chalk Outline) queued its watcher once per card and asked for the
// triggers' order.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestChangesZoneAllGraveSweepFiresOnce: Chalk Outline's "whenever one or
// more creature cards leave your graveyard" queues once for a two-card
// graveyard->exile ChangeZoneAll sweep, not once per card (and therefore no
// trigger-order ask while draining).
func TestChangesZoneAllGraveSweepFiresOnce(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	eng, cfg := miscHandsEngine(t, reg, []string{"Grizzly Bears", "Grizzly Bears"}, nil,
		[]string{"Chalk Outline"}, nil)
	if o := eng.G.Obj(miscBoardObj(t, eng, 0, "Chalk Outline")); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("Chalk Outline is not on the battlefield (vacuous setup)")
	}
	// PRECONDITION: two creature cards in seat 0's graveyard, swept from there.
	var graves []state.ObjID
	for _, id := range eng.G.Zone(state.ZHand, 0) {
		if o := eng.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Grizzly Bears" {
			eng.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZHand, To: state.ZGraveyard})
			eng.pending = nil
			eng.priorityRound()
			graves = append(graves, id)
		}
	}
	if len(graves) < 2 {
		t.Fatalf("only %d creature cards in seat 0's graveyard, need 2 (precondition)", len(graves))
	}
	src := miscBoardObj(t, eng, 0, "Chalk Outline")
	eng.priorityRound()
	start := len(eng.L.Events)
	effects.Resolve(eng, &effects.Ctx{Source: src, Controller: 0, ResolvingObj: src},
		&cards.SA{Kind: "DB", API: "ChangeZoneAll", Params: map[string]string{
			"Origin": "Graveyard", "Destination": "Exile", "ChangeType": "Card.YouOwn"}})
	drainMillTrigger(t, eng, 40)
	for _, id := range graves {
		if o := eng.G.Obj(id); o == nil || o.Zone != state.ZExile {
			t.Fatalf("grave card %d not exiled (precondition)", id)
		}
	}
	if got := countPushes(eng, src, start); got != 1 {
		t.Fatalf("Chalk Outline TriggerPush count = %d, want 1 (one sweep is one event)", got)
	}
	if eng.zoneBatchOpen || eng.zoneBatchDepth != 0 {
		t.Fatalf("zone batch left open (depth %d)", eng.zoneBatchDepth)
	}
	replayCheck(t, eng, cfg)
}
