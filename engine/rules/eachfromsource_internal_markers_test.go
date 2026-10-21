package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestTheOzolithIgnoresInternalMarkers is the end-to-end CR 122.1 read: a
// creature carrying only the engine's Shield status marker leaves the
// battlefield, and The Ozolith's EachFromSource copy must place nothing. The
// marker still makes HasCounters true (a separate read, out of scope), so the
// trigger fires and then copies no counter.
func TestTheOzolithIgnoresInternalMarkers(t *testing.T) {
	t.Parallel()
	oz := mustCorpusCardT(t, "The Ozolith")
	leaver := card(t, "Name:Leaving Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 451, oz, leaver)
	ozID := moveSeededCard(t, e, 0, oz, state.ZBattlefield)
	srcID := moveSeededCard(t, e, 0, leaver, state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: srcID, Counter: "Shield", Amount: 1})
	e.pending = nil
	if got := e.G.Obj(srcID).Counter("Shield"); got != 1 {
		t.Fatalf("precondition: leaver Shield = %d, want 1", got)
	}
	if got := e.G.Obj(ozID).Counter("Shield"); got != 0 {
		t.Fatalf("precondition: Ozolith starts with Shield = %d, want 0", got)
	}

	e.emit(events.Event{Kind: events.MoveZone, Obj: srcID, From: state.ZBattlefield, To: state.ZGraveyard})
	e.pending = nil
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)

	if got := e.G.Obj(ozID).Counter("Shield"); got != 0 {
		t.Fatalf("Ozolith Shield = %d, want 0 (regeneration shield is not a counter)", got)
	}
	if got := e.G.Obj(ozID).Counter("P1P1"); got != 0 {
		t.Fatalf("Ozolith P1P1 = %d, want 0 (nothing real was left behind)", got)
	}
	replayCheck(t, e, cfg)
}
