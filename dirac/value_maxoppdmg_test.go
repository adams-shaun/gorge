package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMaxOppDmgTaken: a table reading Count$MaxOppDamageThisTurn allocates
// the pointer-free per-seat damage-taken tally (genPrep.anyDmgTaken) and
// kop.ExMaxOppDamageThisTurn reads the most damage any one living opponent
// was dealt this turn, over the same coldDmgT records as ExDmgOppsTaken, and
// clears it at the turn change (gorge count_cov3.go
// evalCov3HeadMaxOppDamageThisTurn).
func TestMaxOppDmgTaken(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Exprs = []kop.Expr{{Kind: kop.ExVoid}, {Kind: kop.ExMaxOppDamageThisTurn}}
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s
	if s.dmgTakenCold() == nil {
		t.Fatal("precondition: a table reading MaxOppDamageThisTurn did not allocate the cold per-seat tally")
	}
	s.noteDmgTaken(0, 3)
	s.noteDmgTaken(1, 9)
	if got := s.expr(sc.k, 1, evalCtx{ctrl: 0}); got != 9 {
		t.Errorf("ExMaxOppDamageThisTurn = %d, want the opponent's 9", got)
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
	if got := s.expr(sc.k, 1, evalCtx{ctrl: 0}); got != 0 {
		t.Errorf("ExMaxOppDamageThisTurn after the turn change = %d, want 0", got)
	}
}
