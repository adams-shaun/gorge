package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// castAtPlayerInMain1 casts the named corpus spell (seat 0, turn 1 main phase)
// at seat and returns the engine.
func skipStepsCast(t *testing.T, name, mana string, seat state.PlayerID) (*Engine, Config) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e, cfg := addPhaseEngine(t, reg, []*cards.Card{lookup(t, reg, name)}, []*cards.Card{})
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: mana, Amount: 2})
	spell := moveByName(t, e, 0, name, state.ZHand)
	e.pending = nil
	e.priorityRound()
	castSpellAtPlayer(t, e, spell, seat)
	return e, cfg
}

// TestFatigueSkipsTheTargetsNextDrawStepOnce is Step$ Draw (real corpus
// Fatigue): seat 1's draw step of turn 2 is skipped (no StepChange into it, no
// card drawn), the grant is spent, and turn 4's draw step happens.
func TestFatigueSkipsTheTargetsNextDrawStepOnce(t *testing.T) {
	t.Parallel()
	e, cfg := skipStepsCast(t, "Fatigue", "U", 1)
	if got := e.G.PhaseSkips[1]; got != state.SkipDrawNext {
		t.Fatalf("PhaseSkips[1] = %d after Fatigue resolved, want SkipDrawNext (%d)", got, state.SkipDrawNext)
	}
	draws := countSteps(e, state.StepDraw)
	driveToStep(t, e, 2, 1, state.StepMain1)
	if n := countSteps(e, state.StepDraw); n != draws {
		t.Fatalf("turn 2: %d draw steps entered, want %d (the target's draw skipped)", n, draws)
	}
	if len(e.G.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v after the skipped draw step, want it spent", e.G.PhaseSkips)
	}
	driveToStep(t, e, 4, 1, state.StepMain1)
	if n := countSteps(e, state.StepDraw); n != draws+2 {
		t.Fatalf("turn 3 and 4: %d draw steps in all, want %d (seat 0's and seat 1's again)", n, draws+2)
	}
	replayCheck(t, e, cfg)
}

// TestMomentOfSilenceSkipsTheNextCombatThisTurnOnly is Duration$ NextThisTurn
// (real corpus Moment of Silence): cast on its own caster in turn 1's first
// main phase it skips that turn's combat, and only that one.
func TestMomentOfSilenceSkipsTheNextCombatThisTurnOnly(t *testing.T) {
	t.Parallel()
	e, cfg := skipStepsCast(t, "Moment of Silence", "W", 0)
	if got := e.G.PhaseSkips[0]; got != state.SkipCombatTurn {
		t.Fatalf("PhaseSkips[0] = %d after Moment of Silence resolved, want SkipCombatTurn (%d)", got, state.SkipCombatTurn)
	}
	combats := countSteps(e, state.StepBeginCombat)
	driveToStep(t, e, 1, 0, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != combats {
		t.Fatalf("turn 1: %d beginning-of-combat steps, want %d (skipped)", n, combats)
	}
	if len(e.G.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v after the skipped combat, want it spent", e.G.PhaseSkips)
	}
	driveToStep(t, e, 3, 0, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != combats+2 {
		t.Fatalf("turns 2 and 3: %d beginning-of-combat steps, want %d (both combats happen)", n, combats+2)
	}
	replayCheck(t, e, cfg)
}

// TestThisTurnSkipExpiresAtTheTurnChange: a this-turn combat skip cast on the
// OPPONENT (who is not in a combat phase) is dropped when the turn ends, so
// the opponent's own next combat happens.
func TestThisTurnSkipExpiresAtTheTurnChange(t *testing.T) {
	t.Parallel()
	e, _ := skipStepsCast(t, "Moment of Silence", "W", 1)
	if got := e.G.PhaseSkips[1]; got != state.SkipCombatTurn {
		t.Fatalf("PhaseSkips[1] = %d, want SkipCombatTurn", got)
	}
	combats := countSteps(e, state.StepBeginCombat)
	driveToStep(t, e, 2, 1, state.StepMain2)
	if len(e.G.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v in turn 2, want the this-turn grant dropped", e.G.PhaseSkips)
	}
	if n := countSteps(e, state.StepBeginCombat); n != combats+2 {
		t.Fatalf("%d beginning-of-combat steps, want %d (turn 1's and turn 2's)", n, combats+2)
	}
}
