package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDmgMarkLifetime: a mark is the resolving controller's on the player the
// trigger names; two controllers' marks on one player are two doublings; the
// controller's marks end at the cleanup before their next turn and no other's;
// the same controller marking the same player again stalls StallDmgMark.
func TestDmgMarkLifetime(t *testing.T) {
	var s State
	in := &kop.Instr{Obj: kop.RefTriggerPlayer}
	mark := func(ctrl, victim uint8) {
		s.res.e.ctrl, s.res.e.trigPl = ctrl, victim
		s.dmgMarkAdd(nil, in)
	}
	mark(0, 1)
	if s.dmgMarkN(1) != 1 || s.dmgMarkN(0) != 0 {
		t.Fatalf("marks on seat 1 / seat 0 = %d / %d, want 1 / 0", s.dmgMarkN(1), s.dmgMarkN(0))
	}
	mark(1, 1) // a second controller's mark on the same player
	if s.dmgMarkN(1) != 2 {
		t.Fatalf("marks on seat 1 = %d, want 2 (one per controller)", s.dmgMarkN(1))
	}
	s.active = 0 // seat 0's turn: the cleanup before seat 1's turn ends seat 1's marks only
	s.dmgMarkEnd()
	if s.dmgMarkN(1) != 1 {
		t.Fatalf("marks on seat 1 = %d after seat 1's marks ended, want seat 0's alone", s.dmgMarkN(1))
	}
	s.active = 1 // the cleanup before seat 0's turn
	s.dmgMarkEnd()
	if s.dmgMarkN(1) != 0 || s.g.dmk != [dmkBytes]uint8{} {
		t.Fatalf("marks %v remain after both controllers' next turns", s.g.dmk)
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
