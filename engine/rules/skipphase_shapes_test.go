package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The one-shot DB$ SkipPhase shapes (Forge SkipPhaseEffect, CR 614.10): the
// grant is an events.SkipPhase (Amount 0 untap/draw, 1 next combat, 2 next
// combat this turn), the consumer is turn.go's beginTurn (untap) and
// advanceStep tail (draw, combat), and each skip is spent by one Amount -1
// SkipPhase event.

const skipRockSrc = "Name:Rock\nTypes:Artifact\nOracle:x\n"

func skipConsumed(e *Engine, step state.Step) int {
	return countEvents(e, func(ev events.Event) bool {
		return ev.Kind == events.SkipPhase && ev.Amount < 0 && ev.Step == step
	})
}

// TestSkipPhaseNextUntapStep: four seats; seat 2, not the active player, is
// granted "skip your next untap step" during turn 1. Its tapped permanent stays
// tapped through its turn 3 (no Untap event, no untap step entered, the turn
// enters upkeep) and untaps in its turn 7. Seat 3's untap is untouched.
func TestSkipPhaseNextUntapStep(t *testing.T) {
	t.Parallel()
	e := newSeats(t, 4)
	rock := onBoard(t, e, 2, skipRockSrc)
	other := onBoard(t, e, 3, skipRockSrc)
	e.emit(events.Event{Kind: events.Tap, Obj: rock})
	e.emit(events.Event{Kind: events.Tap, Obj: other})
	driveToStep(t, e, 1, 0, state.StepMain1)
	e.emit(events.Event{Kind: events.SkipPhase, Player: 2, Step: state.StepUntap})
	if got := e.G.PhaseSkips[2]; got != state.SkipUntapPending {
		t.Fatalf("PhaseSkips[2] = %d, want the pending untap skip (%d)", got, state.SkipUntapPending)
	}
	if e.G.PhaseSkips[0] != 0 {
		t.Fatalf("the grant leaked to the active seat: %v", e.G.PhaseSkips)
	}
	unt := func(id state.ObjID) int {
		return countEvents(e, func(ev events.Event) bool { return ev.Kind == events.Untap && ev.Obj == id })
	}
	untaps := countSteps(e, state.StepUntap)
	driveToStep(t, e, 3, 2, state.StepUpkeep)
	if unt(rock) != 0 || !e.G.Obj(rock).Tapped {
		t.Fatalf("seat 2's permanent untapped in its skipped untap step (Untap events %d)", unt(rock))
	}
	if n := countSteps(e, state.StepUntap) - untaps; n != 1 {
		t.Fatalf("%d untap steps entered over turns 2 and 3, want 1 (seat 1's; seat 2's skipped)", n)
	}
	if skipConsumed(e, state.StepUntap) != 1 || len(e.G.PhaseSkips) != 0 {
		t.Fatalf("consume events %d, PhaseSkips %v: want one consumption and the record spent", skipConsumed(e, state.StepUntap), e.G.PhaseSkips)
	}
	driveToStep(t, e, 4, 3, state.StepUpkeep)
	if unt(other) != 1 || e.G.Obj(other).Tapped {
		t.Fatalf("seat 3's untap ran %d times (tapped %v), want once: the skip is seat 2's only", unt(other), e.G.Obj(other).Tapped)
	}
	driveToStep(t, e, 7, 2, state.StepUpkeep)
	if unt(rock) != 1 || e.G.Obj(rock).Tapped {
		t.Fatalf("seat 2's permanent not untapped in its next untap step (Untap events %d)", unt(rock))
	}
}

// TestSkipPhaseNextDrawStep: four seats and real Fatigue. Seat 0 casts it at
// seat 2 in turn 1; seat 2's draw step in turn 3 does not happen (no
// StepChange to draw, no draw, the walk goes upkeep to main phase) and its turn
// 7 draws again. A bare grant to the ACTIVE seat during its upkeep skips that
// very turn's draw ("this turn" for Elfhame Sanctuary).
func TestSkipPhaseNextDrawStep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	m := lookup(t, reg, "Mountain")
	deck := func(extra ...*cards.Card) []*cards.Card {
		d := append([]*cards.Card{}, extra...)
		for len(d) < 40 {
			d = append(d, m)
		}
		return d
	}
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b", "c", "d"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck(lookup(t, reg, "Fatigue")), deck(), deck(), deck()}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "U", Amount: 3})
	fatigue := moveByName(t, e, 0, "Fatigue", state.ZHand)
	e.pending = nil
	e.priorityRound()
	castSpellAtPlayer(t, e, fatigue, 2)
	if got := e.G.PhaseSkips[2]; got != state.SkipDrawPending {
		t.Fatalf("PhaseSkips[2] = %d after Fatigue resolved, want the pending draw skip (%d)", got, state.SkipDrawPending)
	}
	hand := len(e.G.Zone(state.ZHand, 2))
	draws := countSteps(e, state.StepDraw)
	driveToStep(t, e, 3, 2, state.StepMain1)
	if got := len(e.G.Zone(state.ZHand, 2)) - hand; got != 0 {
		t.Fatalf("seat 2's hand grew by %d in its skipped draw step, want 0", got)
	}
	// turns 2 (seat 1) entered its draw step; turn 3 (seat 2) did not
	if n := countSteps(e, state.StepDraw) - draws; n != 1 {
		t.Fatalf("%d draw steps entered over turns 2 and 3, want 1 (seat 1's)", n)
	}
	if skipConsumed(e, state.StepDraw) != 1 || len(e.G.PhaseSkips) != 0 {
		t.Fatalf("consume events %d, PhaseSkips %v", skipConsumed(e, state.StepDraw), e.G.PhaseSkips)
	}
	driveToStep(t, e, 7, 2, state.StepMain1)
	if got := len(e.G.Zone(state.ZHand, 2)) - hand; got != 1 {
		t.Fatalf("seat 2's hand grew by %d by turn 7, want 1 (turn 7's draw; turn 3's skipped)", got)
	}
	// seat 3, active in turn 8, is granted the skip in its own upkeep
	driveToStep(t, e, 8, 3, state.StepUpkeep)
	hand3 := len(e.G.Zone(state.ZHand, 3))
	e.emit(events.Event{Kind: events.SkipPhase, Player: 3, Step: state.StepDraw})
	driveToStep(t, e, 8, 3, state.StepMain1)
	if got := len(e.G.Zone(state.ZHand, 3)) - hand3; got != 0 {
		t.Fatalf("seat 3 drew %d in the draw step it skipped this turn", got)
	}
	replayCheck(t, e, cfg)
}

// TestSkipPhaseNextCombatThisTurn: four seats and real Moment of Silence
// ("skips their next combat phase this turn"). Seat 0 casts it at itself in its
// main phase: this turn's combat is skipped and the other seats' combats are
// not. A this-turn grant to a seat that is not the active player expires at
// the turn boundary (seat 1's turn 2 keeps its combat), while the plain "next
// combat phase" grant (Amount 1: Blinding Angel, Stonehorn Dignitary) to seat
// 2 waits and skips seat 2's turn 3 combat only.
func TestSkipPhaseNextCombatThisTurn(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	m := lookup(t, reg, "Mountain")
	deck := func(extra ...*cards.Card) []*cards.Card {
		d := append([]*cards.Card{}, extra...)
		for len(d) < 40 {
			d = append(d, m)
		}
		return d
	}
	cfg := seatZeroStart(Config{Seed: 42, Names: []string{"a", "b", "c", "d"}, Tokens: reg.Tokens,
		Decks: [][]*cards.Card{deck(lookup(t, reg, "Moment of Silence")), deck(), deck(), deck()}})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "W", Amount: 1})
	moment := moveByName(t, e, 0, "Moment of Silence", state.ZHand)
	e.pending = nil
	e.priorityRound()
	// a this-turn skip for a seat that is not active, and a next-combat skip
	e.emit(events.Event{Kind: events.SkipPhase, Player: 1, Step: state.StepBeginCombat, Amount: 2})
	e.emit(events.Event{Kind: events.SkipPhase, Player: 2, Step: state.StepBeginCombat, Amount: 1})
	castSpellAtPlayer(t, e, moment, 0)
	if got := e.G.PhaseSkips[0]; got != state.SkipCombatThisTurn {
		t.Fatalf("PhaseSkips[0] = %d after Moment of Silence resolved, want the this-turn skip (%d)", got, state.SkipCombatThisTurn)
	}
	driveToStep(t, e, 1, 0, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 0 {
		t.Fatalf("turn 1: %d beginning-of-combat steps, want 0 (skipped this turn)", n)
	}
	if skipConsumed(e, state.StepBeginCombat) != 1 || e.G.PhaseSkips[0] != 0 {
		t.Fatalf("consumed %d, PhaseSkips[0] = %d", skipConsumed(e, state.StepBeginCombat), e.G.PhaseSkips[0])
	}
	driveToStep(t, e, 2, 1, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 1 {
		t.Fatalf("turn 2 (seat 1, whose this-turn grant expired): %d beginning-of-combat steps in all, want 1", n)
	}
	if e.G.PhaseSkips[1] != 0 {
		t.Fatalf("seat 1's this-turn grant survived the turn boundary: %v", e.G.PhaseSkips)
	}
	driveToStep(t, e, 3, 2, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 1 {
		t.Fatalf("turn 3 (seat 2's skipped next combat): %d beginning-of-combat steps in all, want still 1", n)
	}
	driveToStep(t, e, 4, 3, state.StepMain2)
	driveToStep(t, e, 5, 0, state.StepMain2)
	if n := countSteps(e, state.StepBeginCombat); n != 3 {
		t.Fatalf("turns 4 and 5: %d beginning-of-combat steps in all, want 3", n)
	}
	if len(e.G.PhaseSkips) != 0 {
		t.Fatalf("PhaseSkips = %v, want every skip spent", e.G.PhaseSkips)
	}
	replayCheck(t, e, cfg)
}
