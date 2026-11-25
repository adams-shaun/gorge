package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A phased-out permanent is treated as though it does not exist (CR
// 702.26b), so Kirol, History Buff's own "becomes prepared" trigger,
// resolving after Kirol phased out, prepares nothing and mints no exile copy
// (Forge's AlterAttributeEffect skips a phased-out card). Four-seat edh pod
// 72 of the heuristic sweep diverged on the copy gorge minted.
func TestPhasedOutPermanentDoesNotBecomePrepared(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Kirol, History Buff"), bears}, nil)
	kirol := moveByName(t, e, 0, "Kirol, History Buff", state.ZBattlefield)
	bear := moveByName(t, e, 0, "Grizzly Bears", state.ZGraveyard)
	e.checkStateBased()
	e.emit(events.Event{Kind: events.MoveZone, Obj: bear, From: state.ZGraveyard, To: state.ZExile})
	e.priorityRound()
	if len(e.G.Stack) == 0 {
		t.Fatal("precondition: a card left the graveyard and Kirol's trigger is not on the stack")
	}
	e.emit(events.Event{Kind: events.PhaseOut, Obj: kirol, Amount: 1})
	if !e.G.Obj(kirol).PhasedOut {
		t.Fatal("precondition: Kirol did not phase out")
	}
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(kirol).Prepared {
		t.Error("a phased-out Kirol became prepared")
	}
	for _, id := range e.G.Zone(state.ZExile, 0) {
		if o := e.G.Obj(id); o != nil && o.IsCopy && o.PreparedSource != 0 {
			t.Errorf("a prepared exile copy %v was minted for a phased-out source", id)
		}
	}
}
