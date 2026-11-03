package rules

// A cast reversed under CR 733.1 was never cast: its PutOnStack stays in the
// append-only log, but no per-turn cast count may see it. A Spree cast whose
// chosen modes the pool cannot pay is reversed after the mode answer (Phantom
// Interference's {3} mode in a real game); under High Noon ("each player can't
// cast more than one spell each turn") the reversal must not use up the
// allowance, and Storm's count of spells cast this turn must not see it either.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestReversedCastIsNoCastForTheTurnsCounts(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := crAbortEngine(t, reg, "uw-control", "Requisition Raid", "Sol Ring", "Ghostly Prison", "High Noon", "Ornithopter")
	id := crAbortMove(t, e, 0, "Requisition Raid", state.ZHand)
	crAbortMove(t, e, 0, "Sol Ring", state.ZBattlefield)
	crAbortMove(t, e, 0, "Ghostly Prison", state.ZBattlefield)
	crAbortMove(t, e, 0, "High Noon", state.ZBattlefield)
	ornithopter := crAbortMove(t, e, 0, "Ornithopter", state.ZHand)
	tapEveryManaSource(t, e, 0)
	for range 2 {
		e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	}
	e.askPriority(0)
	crAbortAnswer(t, e, "Requisition Raid", crAbortOption(t, e, "Requisition Raid", "cast", id))
	d := e.Pending()
	crAbortAnswer(t, e, "Requisition Raid",
		modeOptionContaining(t, d, "Destroy target artifact."),
		modeOptionContaining(t, d, "Destroy target enchantment."))
	if got := e.G.Obj(id).Zone; got != state.ZHand {
		t.Fatalf("over-budget Requisition Raid zone = %s, want hand (the fixture must reverse)", got)
	}
	puts := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.PutOnStack && ev.Obj == id {
			puts++
		}
	}
	if puts != 1 {
		t.Fatalf("Requisition Raid PutOnStack events = %d, want the one the reversal left in the log", puts)
	}
	if got := e.CastThisTurn(); got != 0 {
		t.Errorf("CastThisTurn after a reversed cast = %d, want 0", got)
	}
	if got := e.SpellsCastThisTurnMatching(0, "Card.YouCtrl", effects.TriggerContext{}); got != 0 {
		t.Errorf("SpellsCastThisTurnMatching after a reversed cast = %d, want 0", got)
	}
	// High Noon allows one spell each turn: the reversed cast is not it.
	crAbortOption(t, e, "Ornithopter", "cast", ornithopter)
}
