package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// restrN is how much restricted mana p has floating; restrOf is its units of
// colour c.
func (s *State) restrN(p uint8) int { return s.rbTotal(p, s.rbAll(p)) }

func (s *State) restrOf(p uint8, c int) int {
	var t [kop.NumManaKinds]uint8
	s.rbUnits(p, s.rbAll(p), &t)
	return int(t[c])
}

// setRestr floats n restricted mana of colour c of batch kind kind for p, alone.
func (s *State) setRestr(p, kind uint8, c int, n uint8) {
	s.rbClear(p)
	s.rb[p&1][0] = rbatch{kind: kind}
	s.rb[p&1][0].n[c] = n
}

// TestRestrictedBatchOrder: a unit joins the latest batch of its restriction
// unless a later batch holds its colour, so the batches keep gorge's insertion
// order where a payment admitting both could tell them apart.
func TestRestrictedBatchOrder(t *testing.T) {
	var s State
	if !s.rbPut(0, 1, 0, kop.W, 1) || !s.rbPut(0, 2, 0, kop.U, 1) || !s.rbPut(0, 1, 0, kop.G, 1) {
		t.Fatal("setup: the puts failed")
	}
	if s.rb[0][0].n[kop.G] != 1 || s.rb[0][2].kind != 0 {
		t.Fatalf("a colour no later batch holds merged into a new batch: %v", s.rb[0])
	}
	if !s.rbPut(0, 2, 0, kop.W, 1) || !s.rbPut(0, 1, 0, kop.W, 1) {
		t.Fatal("setup: the puts failed")
	}
	if s.rb[0][1].n[kop.W] != 1 || s.rb[0][2].kind != 1 || s.rb[0][2].n[kop.W] != 1 {
		t.Fatalf("a {W} after another restriction's {W} merged ahead of it: %v", s.rb[0])
	}
	if !s.rbPut(0, 3, 0, kop.R, 1) || s.rbPut(0, 4, 0, kop.R, 1) {
		t.Fatalf("the fourth batch did not take, or a fifth did: %v", s.rb[0])
	}
	if s.rbSpendN(0, 1<<1|1<<3, 9) != 3 || s.rb[0][1].kind != 1 || s.rb[0][2].kind != 0 {
		t.Fatalf("spending batches 1 and 3 did not drop them in order: %v", s.rb[0])
	}
}
