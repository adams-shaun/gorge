package dirac

import "testing"

// TestMaxOppDmgTaken: the per-seat damage-taken tally feeds
// Count$MaxOppDamageThisTurn (gorge evalCov3HeadMaxOppDamageThisTurn): the
// most damage any one living opponent was dealt this turn, over the same
// coldDmgT records as ExDmgOppsTaken, and cleared at the turn change.
func TestMaxOppDmgTaken(t *testing.T) {
	var s State
	var l coldLayout
	coldDmgT.set(&l, MaxSeats)
	l.size()
	s.layOut(&l)
	if s.dmgTakenCold() == nil {
		t.Fatal("precondition: the cold per-seat tally was not laid out")
	}
	s.noteDmgTaken(0, 3)
	s.noteDmgTaken(1, 9)
	if got := s.maxOppDmgTaken(0); got != 9 {
		t.Errorf("maxOppDmgTaken(0) = %d, want the opponent's 9", got)
	}
	if got := s.maxOppDmgTaken(1); got != 3 {
		t.Errorf("maxOppDmgTaken(1) = %d, want the opponent's 3", got)
	}

	// A departed seat's damage is not an opponent's any more (CR 800.4a):
	// four seats, seat 2 lost after being dealt 20, seat 1 still has 4.
	s.dmgTakenNewTurn()
	s.g.nseat = 4
	s.noteDmgTaken(1, 4)
	s.noteDmgTaken(2, 20)
	s.pl[2].lost = true
	if got := s.maxOppDmgTaken(0); got != 4 {
		t.Errorf("maxOppDmgTaken(0) with a departed seat = %d, want 4", got)
	}

	s.dmgTakenNewTurn()
	if got := s.maxOppDmgTaken(0); got != 0 {
		t.Errorf("maxOppDmgTaken after the turn change = %d, want 0", got)
	}
}
