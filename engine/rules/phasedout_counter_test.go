package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestCR702PhasedOutPermanentTakesNoCounter pins CR 702.26b: a phased-out
// permanent can't be affected by anything, so a counter placement aimed at it
// (a Spacecraft's station ability resolving after U.S.S. Defiant phased the
// Spacecraft out) puts nothing on it. The phased-in placement on the same
// object is the control.
func TestCR702PhasedOutPermanentTakesNoCounter(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 503, phasedManaArtifact)
	id := moveSeededCard(t, e, 0, card(t, phasedManaArtifact), state.ZBattlefield)
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 2})
	if got := e.G.Obj(id).Counter("CHARGE"); got != 2 {
		t.Fatalf("precondition: a phased-in permanent took %d charge counters, want 2", got)
	}
	e.emit(events.Event{Kind: events.PhaseOut, Obj: id, Amount: 1})
	if !e.G.Obj(id).PhasedOut {
		t.Fatal("precondition: the artifact was not phased out")
	}
	e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "CHARGE", Amount: 4})
	if got := e.G.Obj(id).Counter("CHARGE"); got != 2 {
		t.Fatalf("CR 702.26b: a phased-out permanent took counters: %d charge, want 2", got)
	}
}
