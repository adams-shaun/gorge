package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestIngestExilesTopCard is CR 702.115a: Mist Intruder's Ingest, "whenever
// this creature deals combat damage to a player, that player exiles the top
// card of their library". The defender's top card moves to exile, exactly one.
func TestIngestExilesTopCard(t *testing.T) {
	t.Parallel()
	e, cfg := combatTriggerBoard(t, testutil.CorpusRegistry(t), []string{"Mist Intruder"}, nil, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	id := findBattlefield(t, e, 0, "Mist Intruder", 0)
	lib := e.G.Zone(state.ZLibrary, 1)
	if len(lib) == 0 {
		t.Fatal("precondition: the defender's library is empty")
	}
	top, exiled := lib[0], len(e.G.Zone(state.ZExile, 1))
	e.askAttackers()
	submitAttackers(t, e, id)
	drainCombatDamagePriority(t, e)
	passUntilStackEmpty(t, e, 30)
	if o := e.G.Obj(top); o == nil || o.Zone != state.ZExile {
		t.Fatalf("the defender's top card %d is in %v, want exile", top, e.G.Obj(top).Zone)
	}
	if n := len(e.G.Zone(state.ZExile, 1)) - exiled; n != 1 {
		t.Fatalf("Ingest exiled %d cards, want 1", n)
	}
	replayCheck(t, e, cfg)
}
