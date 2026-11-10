package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDmgMarkControllerLeaves: a controller that leaves the game keeps the
// boundary frozen at the last cleanup it was in (gorge nextTurnFor returns 0
// for it, rescheduleNextTurnBoundaries leaves UntilTurn), and its mark ends at
// that cleanup all the same, not never. Four seats only.
func TestDmgMarkControllerLeaves(t *testing.T) {
	if MaxSeats == 2 {
		t.Skip("at two seats a loss ends the game")
	}
	var s State
	s.g.nseat = 4
	// Seat 2 marks seat 3 in seat 0's turn: its next turn is two after
	// this one, so the mark ends at the cleanup of seat 1's turn.
	s.active = 0
	s.res.e.ctrl, s.res.e.trigPl = 2, 3
	s.dmgMarkAdd(nil, &kop.Instr{Obj: kop.RefTriggerPlayer})
	if !dmkSet(&s, 2, 3) {
		t.Fatal("no mark made")
	}
	s.dmgMarkEnd() // seat 0's cleanup: not yet
	if !dmkSet(&s, 2, 3) {
		t.Fatal("mark ended at the first cleanup, two turns early")
	}
	seat := uint8(2)
	s.pl[seat].lost = true // seat 2 leaves in seat 1's turn
	s.active = 1
	s.dmgMarkEnd() // seat 1's cleanup: the frozen boundary
	if dmkSet(&s, 2, 3) {
		t.Fatal("a departed controller's mark outlived its boundary")
	}
}

// TestDmgMarkControllerLeavesEarly: seat 3 marks seat 1 in seat 0's turn and
// leaves in that same turn, before any cleanup re-derived its boundary: the
// mark survives the cleanups of seats 0 and 1 and ends at seat 2's, the turn
// before seat 3's.
func TestDmgMarkControllerLeavesEarly(t *testing.T) {
	if MaxSeats == 2 {
		t.Skip("at two seats a loss ends the game")
	}
	var s State
	s.g.nseat = 4
	s.active = 0
	s.res.e.ctrl, s.res.e.trigPl = 3, 1
	s.dmgMarkAdd(nil, &kop.Instr{Obj: kop.RefTriggerPlayer})
	seat := uint8(3)
	s.pl[seat].lost = true
	for a := uint8(0); a < 3; a++ {
		s.active = a
		s.dmgMarkEnd()
		if got := dmkSet(&s, 3, 1); got != (a < 2) {
			t.Fatalf("after the cleanup of seat %d's turn the mark is live=%v, want %v", a, got, a < 2)
		}
	}
}
