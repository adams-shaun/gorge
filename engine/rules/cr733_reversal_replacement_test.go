package rules

// CR 733.1: reversing an illegal cast returns the spell to the zone it came
// from, and no effects apply as a result of the undone action. A graveyard
// cast reversed while a "would be put into a graveyard -> exile" replacement
// is active must therefore still return to the graveyard: the compensating
// move is an undo, not a new zone change.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestReversedGraveyardCastReturnsPastGraveyardExileReplacement(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	// Abandoned Sarcophagus grants casting cycling cards from the graveyard
	// and replaces their graveyard moves with exile; Angelsong is a plain
	// no-target cycling instant, so the reversal is an unpayable-mana abort.
	e := crAbortEngine(t, reg, "uw-control", "Abandoned Sarcophagus", "Angelsong", "Angelsong")
	id := crAbortMove(t, e, 0, "Angelsong", state.ZGraveyard)
	crAbortMove(t, e, 0, "Abandoned Sarcophagus", state.ZBattlefield)
	// Precondition: the replacement is live, so an ordinary move of another
	// cycling card into this graveyard is exiled instead of stored.
	probe := crAbortMove(t, e, 0, "Angelsong", state.ZHand)
	e.emit(events.Event{Kind: events.MoveZone, Obj: probe, From: state.ZHand, To: state.ZGraveyard})
	if got := e.G.Obj(probe).Zone; got != state.ZExile {
		t.Fatalf("precondition: probe cycling card moved to %s, want exile (the replacement must be live)", got)
	}
	tapEveryManaSource(t, e, 0)
	e.askPriority(0)
	e.pending = nil // Direct proposal exercises the unpayable-mana abort site.
	e.beginCast(0, decision.Option{Kind: "cast", Obj: id})
	e.Advance()
	if !hasNote(e, "cast aborted: cost no longer payable") {
		t.Fatalf("precondition: the cast did not abort at the unpayable-mana site (card rests in %s)", e.G.Obj(id).Zone)
	}
	if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
		t.Fatalf("CR 733.1: reversed graveyard cast rests in %s, want the graveyard it came from", got)
	}
}
