package view

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// roundTestGame builds a fresh game of n seats starting at startingLife, with
// the given seats marked Lost (eliminated). roundOf reads only g.Turn and the
// Lost flags, so this is the whole projection's input.
func roundTestGame(n int, life int32, lost ...state.PlayerID) *state.Game {
	g := state.NewGameLife(make([]string, n), life)
	for _, p := range lost {
		g.Players[p].Lost = true
	}
	return g
}

func TestRoundOfCountsFullPassesBeforeAnyElimination(t *testing.T) {
	// Four seats, nobody eliminated: the round is exactly the number of
	// complete passes around the table. Turn 1-4 all happen inside round 1
	// (the first full circuit), turn 5 opens round 2, and so on.
	g := roundTestGame(4, 20)
	for _, tc := range []struct {
		turn, want int32
	}{
		{1, 1}, {2, 1}, {3, 1}, {4, 1},
		{5, 2}, {6, 2}, {7, 2}, {8, 2},
		{9, 3}, {10, 3}, {11, 3}, {12, 3},
		{13, 4},
	} {
		g.Turn = tc.turn
		if got := roundOf(g); got != tc.want {
			t.Fatalf("turn %d: roundOf = %d, want %d", tc.turn, got, tc.want)
		}
	}
}

func TestRoundOfRoundLengthShrinksWithElimination(t *testing.T) {
	// A four-seat table where one seat is already gone has a round length of
	// three: turns 1-3 are round 1, turn 4 opens round 2, turn 7 opens round
	// 3. This is the "does the round length shrink with it" question -- the
	// round-trip is a pass of the seats still at the table, so yes.
	g := roundTestGame(4, 20, 2)
	for _, tc := range []struct {
		turn, want int32
	}{
		{1, 1}, {2, 1}, {3, 1},
		{4, 2}, {5, 2}, {6, 2},
		{7, 3}, {8, 3}, {9, 3},
		{10, 4},
	} {
		g.Turn = tc.turn
		if got := roundOf(g); got != tc.want {
			t.Fatalf("turn %d: roundOf = %d, want %d", tc.turn, got, tc.want)
		}
	}
}

// TestRoundOfNeverJumpsBackwardsOrRepeatsAtElimination is the pin for the
// elimination behaviour the brief singles out: a viewer must never read a
// round number that is lower than one they already saw, nor re-read a later
// game state with an earlier round. It simulates a full match: turns advance
// one at a time, and seats are eliminated at turns where the engine would
// have a seat lose. The projected round must be monotonic non-decreasing
// across the whole run.
func TestRoundOfNeverJumpsBackwardsOrRepeatsAtElimination(t *testing.T) {
	g := roundTestGame(4, 20)
	last := int32(0)
	// A hand-written elimination schedule: seat 2 loses during its turn 3
	// (round 1), seat 1 loses during its turn 6 (round 2). This is a reachable
	// shape -- life loss/commander damage eliminate a seat mid-round.
	eliminateAt := map[int32]state.PlayerID{3: 2, 6: 1}
	for turn := int32(1); turn <= 14; turn++ {
		if p, ok := eliminateAt[turn]; ok {
			g.Players[p].Lost = true
		}
		g.Turn = turn
		r := roundOf(g)
		// The clock must never go backwards: a viewer who read "Round 7"
		// must never be shown "Round 6" for a later game state, whatever
		// happened to the seats in between.
		if r < last {
			t.Fatalf("turn %d: round fell from %d to %d -- the clock jumped backwards", turn, last, r)
		}
		last = r
	}
	// And the final value is where a shrinking round length puts it: at turn
	// 14 with two seats still alive, round = (13)/2 + 1 = 7.
	if last != 7 {
		t.Fatalf("final round = %d, want 7", last)
	}
}

func TestRoundOfGuards(t *testing.T) {
	// Pre-genesis (Turn 0), a nil game and an all-eliminated table all
	// project to round 1 rather than dividing by zero or panicking.
	if got := roundOf(state.NewGameLife([]string{"a", "b"}, 20)); got != 1 {
		t.Fatalf("turn 0: roundOf = %d, want 1", got)
	}
	if got := roundOf(nil); got != 1 {
		t.Fatalf("nil game: roundOf = %d, want 1", got)
	}
	g := roundTestGame(4, 20, 0, 1, 2, 3)
	g.Turn = 9
	if got := roundOf(g); got != 1 {
		t.Fatalf("all eliminated: roundOf = %d, want 1", got)
	}
}

// TestViewCarriesRound pins that the wire View actually receives the
// projection, not just the helper in isolation.
func TestViewCarriesRound(t *testing.T) {
	g := roundTestGame(4, 20)
	g.Turn = 9
	// 9th turn, four alive: round 3 (turns 7-9 are the start of round 3).
	v := ProjectFor(g, nil, state.PlayerID(0), Seat, nil)
	if v.Round != 3 {
		t.Fatalf("View.Round = %d, want 3", v.Round)
	}
}
