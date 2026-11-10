package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// dmkSet reports whether the pair (controller c, player p) is marked.
func dmkSet(s *State, c, p uint8) bool {
	i, m := dmkBit(c, p)
	return s.g.dmk[i]&m != 0
}

// TestDmgMarkLifetime: a mark is the resolving controller's on the player the
// trigger names; two controllers' marks on one player are two doublings; the
// controller's marks end at the cleanup before their next turn and no other's
// (until your next turn, not until end of turn: the mark made in its
// controller's turn survives that turn's cleanup); the same controller marking
// the same player again stalls StallDmgMark.
func TestDmgMarkLifetime(t *testing.T) {
	var s State
	in := &kop.Instr{Obj: kop.RefTriggerPlayer}
	mark := func(ctrl, victim uint8) {
		s.res.e.ctrl, s.res.e.trigPl = ctrl, victim
		s.dmgMarkAdd(nil, in)
	}
	s.g.nseat = MaxSeats
	last := uint8(MaxSeats - 1) // the seat whose turn precedes seat 0's
	s.active = 0
	mark(0, 1)
	if s.dmgMarkN(1) != 1 || s.dmgMarkN(0) != 0 {
		t.Fatalf("marks on seat 1 / seat 0 = %d / %d, want 1 / 0", s.dmgMarkN(1), s.dmgMarkN(0))
	}
	mark(1, 1) // a second controller's mark on the same player
	if s.dmgMarkN(1) != 2 {
		t.Fatalf("marks on seat 1 = %d, want 2 (one per controller)", s.dmgMarkN(1))
	}
	// Seat 0's turn is ending: the next cleanup-before-a-turn is seat 1's.
	s.dmgMarkEnd()
	if !dmkSet(&s, 0, 1) || dmkSet(&s, 1, 1) {
		t.Fatalf("after the cleanup of seat 0's turn: seat 0's mark %v (want kept), seat 1's %v (want ended)", dmkSet(&s, 0, 1), dmkSet(&s, 1, 1))
	}
	s.active = last // the last seat's turn is ending: seat 0's next turn follows
	s.dmgMarkEnd()
	if s.dmgMarkN(1) != 0 || s.g.dmk != [dmkBytes]uint8{} {
		t.Fatalf("marks %v remain after both controllers' next turns", s.g.dmk)
	}
	mark(0, 1) // made in the last seat's turn: ends at its cleanup (seat 0's next turn follows)
	s.dmgMarkEnd()
	if s.g.dmk != [dmkBytes]uint8{} {
		t.Fatalf("marks %v remain after the cleanup before their controller's turn", s.g.dmk)
	}
	mark(0, 1)
	mark(0, 1)
	if s.g.stalled != StallDmgMark {
		t.Fatalf("stalled %d marking one player twice, want %d", s.g.stalled, StallDmgMark)
	}
}

// TestDmgMarkCompetition: an event the mark and one other replacement match
// stalls (gorge asks their order); the mark alone, or another alone, does not.
func TestDmgMarkCompetition(t *testing.T) {
	var s State
	s.res.e.ctrl, s.res.e.trigPl = 0, 1
	s.dmgMarkAdd(nil, &kop.Instr{Obj: kop.RefTriggerPlayer})
	if s.dmgMarkCompetes(1, 0) || s.dmgMarkCompetes(0, 5) || s.g.stalled != 0 {
		t.Fatalf("stalled %d with no competition", s.g.stalled)
	}
	if !s.dmgMarkCompetes(1, 1) || s.g.stalled != StallDmgMarkOrder {
		t.Fatalf("stalled %d beside another replacement, want %d", s.g.stalled, StallDmgMarkOrder)
	}
}
