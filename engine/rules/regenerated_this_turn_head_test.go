package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRegeneratedThisTurnHeadCountsRegenerations: Spiny Starfish's
// Count$RegeneratedThisTurn is the number of times the source regenerated
// since the turn began (CR 701.19), reset at the turn boundary.
func TestRegeneratedThisTurnHeadCountsRegenerations(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	star := historyCorpusToZone(t, e, 0, "Spiny Starfish", state.ZBattlefield)
	other := historyCorpusToZone(t, e, 0, "Spiny Starfish", state.ZBattlefield)
	const expr = "Count$RegeneratedThisTurn"
	if body := e.G.Obj(star).Face().SVars["X"]; body != expr {
		t.Fatalf("corpus body = %q", body)
	}
	if n := historyEval(t, e, star, nil, expr); n != 0 {
		t.Fatalf("before any regeneration = %d, want 0", n)
	}
	regen := func(id state.ObjID) {
		t.Helper()
		e.emit(events.Event{Kind: events.CounterChange, Obj: id, Counter: "Shield", Amount: 1})
		if !effects.ReplaceDestruction(e, id) {
			t.Fatal("the shield did not replace the destruction")
		}
	}
	regen(star)
	if n := historyEval(t, e, star, nil, expr); n != 1 {
		t.Fatalf("after one regeneration = %d, want 1", n)
	}
	regen(star)
	if n := historyEval(t, e, star, nil, expr); n != 2 {
		t.Fatalf("after two regenerations = %d, want 2", n)
	}
	if n := historyEval(t, e, other, nil, expr); n != 0 {
		t.Fatalf("a sibling that never regenerated = %d, want 0 (the tally is per object)", n)
	}
	// An unspent shield swept at cleanup is not a regeneration.
	e.emit(events.Event{Kind: events.CounterChange, Obj: other, Counter: "Shield", Amount: 1})
	e.emit(events.Event{Kind: events.CounterChange, Obj: other, Counter: "Shield", Amount: -1})
	if n := historyEval(t, e, other, nil, expr); n != 0 {
		t.Fatalf("an unspent shield counted as a regeneration: %d", n)
	}
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: e.G.Turn + 1})
	if n := historyEval(t, e, star, nil, expr); n != 0 {
		t.Fatalf("after the turn boundary = %d, want 0", n)
	}
}
