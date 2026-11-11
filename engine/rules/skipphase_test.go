package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFalsePeaceSkipsTheTargetsNextCombats is DB$ SkipPhase's leaf (real
// corpus False Peace, CR 614.10): seat 0 casts it at seat 1 in turn 1's main
// phase. Seat 0's own combat that turn, and seat 0's turn 3, are untouched;
// seat 1's turn 2 goes from its precombat main phase to its postcombat main
// phase with no beginning of combat; seat 1's turn 4 has its combat again.
func TestFalsePeaceSkipsTheTargetsNextCombats(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := addPhaseEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "False Peace")}, []*cards.Card{})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	peace := moveByName(t, e, 0, "False Peace", state.ZHand)
	e.pending = nil
	e.priorityRound()
	castSpellAtPlayer(t, e, peace, 1)
	if got := e.G.PhaseSkips[1]; got != state.SkipCombatPending {
		t.Fatalf("PhaseSkips[1] = %d after False Peace resolved, want pending (%d)", got, state.SkipCombatPending)
	}
	if e.G.CombatSkipped(1) || e.G.CombatSkipped(0) {
		t.Fatal("a pending skip must not skip the turn being played")
	}
	driveToStep(t, e, 1, 0, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 1 {
		t.Fatalf("turn 1 (the caster's): %d beginning-of-combat steps, want 1", n)
	}
	driveToStep(t, e, 2, 1, state.StepMain2)
	if !e.G.CombatSkipped(1) {
		t.Fatalf("PhaseSkips[1] = %d in turn 2, want the live skip", e.G.PhaseSkips[1])
	}
	if n := countSteps(e, state.StepBeginCombat); n != 1 {
		t.Fatalf("turn 2 (the target's next turn): %d beginning-of-combat steps in all, want 1 (its combat skipped)", n)
	}
	driveToStep(t, e, 3, 0, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 2 {
		t.Fatalf("turn 3: %d beginning-of-combat steps in all, want 2", n)
	}
	if len(e.G.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v after the skipped turn, want it spent", e.G.PhaseSkips)
	}
	driveToStep(t, e, 4, 1, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 3 {
		t.Fatalf("turn 4 (the target's turn after): %d beginning-of-combat steps in all, want 3", n)
	}
	if got := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.SkipPhase }); got != 1 {
		t.Fatalf("%d SkipPhase events, want 1", got)
	}
	replayCheck(t, e, cfg)
}

// TestFalsePeaceSkipsAnExtraCombatToo: "all combat phases" includes an extra
// one. Seat 1's live skip meets a granted extra combat spliced after the
// precombat main phase: the grant is consumed and completed without the combat
// beginning, and the walk goes on to the postcombat main phase.
func TestFalsePeaceSkipsAnExtraCombatToo(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := addPhaseEngine(t, reg,
		[]*cards.Card{lookup(t, reg, "False Peace")}, []*cards.Card{})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	peace := moveByName(t, e, 0, "False Peace", state.ZHand)
	e.pending = nil
	e.priorityRound()
	castSpellAtPlayer(t, e, peace, 1)
	driveToStep(t, e, 2, 1, state.StepMain1)
	if !e.G.CombatSkipped(1) {
		t.Fatalf("precondition: PhaseSkips[1] = %d in turn 2, want the live skip", e.G.PhaseSkips[1])
	}
	before := countSteps(e, state.StepBeginCombat)
	e.emit(events.Event{Kind: events.ExtraPhase, Player: 1, Amount: 1, Step: state.StepMain1,
		IDs: []state.ObjID{state.ObjID(state.StepBeginCombat)}})
	if len(e.G.ExtraPhases) != 1 {
		t.Fatalf("precondition: ExtraPhases = %+v, want the one extra combat grant", e.G.ExtraPhases)
	}
	driveToStep(t, e, 2, 1, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat) - before; n != 0 {
		t.Fatalf("%d beginning-of-combat steps in the skipping player's turn, want none", n)
	}
	if len(e.G.ExtraPhases) != 0 {
		t.Fatalf("ExtraPhases = %+v, want the skipped grant consumed and completed", e.G.ExtraPhases)
	}
	if got := countEvents(e, func(ev events.Event) bool { return ev.Kind == events.ExtraPhase && ev.Amount < 0 }); got != 2 {
		t.Fatalf("%d ExtraPhase consume/complete events, want 2", got)
	}
	replayCheck(t, e, cfg)
}

// castSpellAtPlayer casts id, answers its target decision with the player
// seat, and drains the stack.
func castSpellAtPlayer(t *testing.T, e *Engine, id state.ObjID, seat state.PlayerID) {
	t.Helper()
	cast := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "cast" && o.Obj == id {
			cast = o.Index
		}
	}
	if cast < 0 {
		t.Fatalf("no cast option for %d", id)
	}
	submitChoices(t, e, cast)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected a target decision, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Kind == "player" && o.Player == seat {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("seat %d not offered as a target", seat)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 20)
}
