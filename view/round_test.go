package view

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
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

// turnLog builds a synthetic ordered event stream of TurnChange events for
// the given seats, in order. RoundOf reads only these, so a hand-built log
// is a complete input.
func turnLog(seats ...state.PlayerID) []events.Event {
	ev := make([]events.Event, 0, len(seats))
	for _, s := range seats {
		ev = append(ev, events.Event{Kind: events.TurnChange, Player: s})
	}
	return ev
}

// TestRoundOfFoldsEliminationWithoutJump is the ui13 discrimination proof: a
// game with an elimination where the snapshot-only projection (roundOf, the
// old 1+(Turn-1)/AliveCount) JUMPS and the event-stream fold (RoundOf) does
// not. Four seats play ten full rounds (turns 1-40, all alive), seat 3 loses
// during its own turn 40, and play returns to seat 0 at turn 41. The exact
// round at turn 41 is 11 -- ten rounds done, the eleventh just opened. The
// snapshot projection, reading only the CURRENT alive set (now three seats)
// and the current Turn, computes 1+(41-1)/3 = 14 and jumps three rounds
// ahead of reality, which is exactly the defect ui12 booked.
func TestRoundOfFoldsEliminationWithoutJump(t *testing.T) {
	// Turns 1-40 seat order 0,1,2,3,0,1,2,3,... (ten full rounds).
	var evs []events.Event
	for i := 0; i < 40; i++ {
		evs = append(evs, events.Event{Kind: events.TurnChange, Player: state.PlayerID(i % 4)})
	}
	// Seat 3 loses during its own turn 40; play returns to seat 0 at turn 41.
	evs = append(evs, events.Event{Kind: events.PlayerLost, Player: 3})
	evs = append(evs, events.Event{Kind: events.TurnChange, Player: 0})

	if got := RoundOf(roundTestGame(4, 20), evs); got != 11 {
		t.Fatalf("RoundOf = %d, want 11 (exact: ten rounds plus the eleventh just opened)", got)
	}

	// The snapshot-only projection at the equivalent snapshot (Turn 41, seat
	// 3 gone) reads a CURRENT alive count of three and jumps to 14.
	g := roundTestGame(4, 20, 3)
	g.Turn = 41
	if got := roundOf(g); got != 14 {
		t.Fatalf("roundOf(snapshot) = %d, want 14 to prove the old projection jumps", got)
	}
}

// TestRoundOfFirstRoundHandlesNoTurn is the edge case "the first round,
// before any turn has been taken": no TurnChange seen yet folds to round 1,
// and the very first TurnChange must not itself open a round 2.
func TestRoundOfFirstRoundHandlesNoTurn(t *testing.T) {
	if got := RoundOf(roundTestGame(4, 20), nil); got != 1 {
		t.Fatalf("empty log: RoundOf = %d, want 1", got)
	}
	if got := RoundOf(roundTestGame(4, 20), turnLog(0)); got != 1 {
		t.Fatalf("first TurnChange only: RoundOf = %d, want 1", got)
	}
	if got := RoundOf(roundTestGame(4, 20), turnLog(0, 1, 2, 3)); got != 1 {
		t.Fatalf("first full pass: RoundOf = %d, want 1", got)
	}
}

// TestRoundOfMidRoundEliminationDoesNotWrap is the edge case "a seat
// eliminated mid-round": a death in the MIDDLE of the cycle (not the
// anchor, and not the cycle's end) must not itself open a round -- the
// survivors just get a shorter cycle, and a boundary fires only when play
// genuinely returns to the (unchanged) first survivor.
func TestRoundOfMidRoundEliminationDoesNotWrap(t *testing.T) {
	// Four seats. Round 1 = 0,1,2,3; round 2 opens at the return to seat 0.
	// During seat 1's turn (turn 6, mid-round-2), seat 1 is eliminated -- a
	// NON-anchor death in the middle of the cycle. NextAlive then skips it,
	// so the cycle becomes 0,2,3 and the next return to seat 0 (the still-
	// unchanged first survivor) opens round 3. The death itself adds no
	// boundary.
	evs := []events.Event{
		{Kind: events.TurnChange, Player: 0}, // turn 1
		{Kind: events.TurnChange, Player: 1}, // turn 2
		{Kind: events.TurnChange, Player: 2}, // turn 3
		{Kind: events.TurnChange, Player: 3}, // turn 4
		{Kind: events.TurnChange, Player: 0}, // turn 5: round 2 opens
		{Kind: events.TurnChange, Player: 1}, // turn 6
		{Kind: events.PlayerLost, Player: 1}, // seat 1 eliminated mid-round
		{Kind: events.TurnChange, Player: 2}, // turn 7
		{Kind: events.TurnChange, Player: 3}, // turn 8
		{Kind: events.TurnChange, Player: 0}, // turn 9: round 3 opens
	}
	if got := RoundOf(roundTestGame(4, 20), evs); got != 3 {
		t.Fatalf("mid-round elimination: RoundOf = %d, want 3", got)
	}
}

// TestRoundOfAnchorEliminatedCarriesOn is the edge case "a seat eliminated
// who was the one the count was anchored on". The anchor is the FIRST
// still-living seat, the one the round count is anchored on (rounds open by
// returning to it). Seat 0 is the anchor; when it loses during its own turn
// 1, the next turn passes to seat 1 (the new first survivor) and -- because
// play has returned to the first still-living seat -- that opens round 2.
// This is exactly the case that defeats a naive "the active index fell"
// rule: the turn goes 0 -> 1, an index RISE, yet it is a genuine round
// boundary. The counter carries cleanly across the elimination rather than
// double-counting or skipping.
func TestRoundOfAnchorEliminatedCarriesOn(t *testing.T) {
	evs := []events.Event{
		{Kind: events.TurnChange, Player: 0}, // turn 1: seat 0, the anchor
		{Kind: events.PlayerLost, Player: 0}, // seat 0 eliminated on its turn
		{Kind: events.TurnChange, Player: 1}, // turn 2: round 2 opens at new first survivor
		{Kind: events.TurnChange, Player: 2}, // turn 3
		{Kind: events.TurnChange, Player: 3}, // turn 4
		{Kind: events.TurnChange, Player: 1}, // turn 5: round 3 opens
		{Kind: events.TurnChange, Player: 2}, // turn 6
		{Kind: events.TurnChange, Player: 3}, // turn 7
		{Kind: events.TurnChange, Player: 1}, // turn 8: round 4 opens
	}
	if got := RoundOf(roundTestGame(4, 20), evs); got != 4 {
		t.Fatalf("anchor eliminated: RoundOf = %d, want 4", got)
	}
}

// TestRoundOfMatchesApproximationBeforeElimination is a sanity pin: with no
// elimination the exact fold and the snapshot projection agree (the fold is
// exact where the approximation is exact too, and the two cannot diverge
// before anyone dies).
func TestRoundOfMatchesApproximationBeforeElimination(t *testing.T) {
	g := roundTestGame(4, 20)
	var evs []events.Event
	for turn := int32(1); turn <= 16; turn++ {
		evs = append(evs, events.Event{Kind: events.TurnChange, Player: state.PlayerID((turn - 1) % 4)})
		g.Turn = turn
		if got, want := RoundOf(g, evs), roundOf(g); got != want {
			t.Fatalf("turn %d: exact %d != approximation %d", turn, got, want)
		}
	}
}
