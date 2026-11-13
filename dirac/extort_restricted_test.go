package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExtortPipIgnoresRestrictedMana is the dirac half of gorge's
// TestExtortPipIgnoresRestrictedMana: mana a restriction limits to casts or
// activations (State.rb) cannot pay Extort's {W/B}, so a seat holding only a
// restricted W declines and keeps the batch, and a plain W beside it pays and
// leaves the batch alone.
func TestExtortPipIgnoresRestrictedMana(t *testing.T) {
	prog := []kop.Instr{{Op: kop.OpMill, Obj: kop.RefController, A: kop.Const(1)}}
	sc := czScen(t, prog)
	s := sc.s
	s.pl[0].pool = [kop.NumManaKinds]uint8{}
	s.rb[0][0] = rbatch{kind: 1}
	s.rb[0][0].n[kop.W] = 1
	if got := s.poolProj(0)[kop.W]; got != 1 {
		t.Fatalf("precondition: the restricted W is not floating (poolProj W %d)", got)
	}
	if s.payExtortPip(0) {
		t.Fatal("the restricted W paid Extort's pip")
	}
	if s.rb[0][0].n[kop.W] != 1 {
		t.Fatal("a declined pip spent the restricted unit")
	}
	s.pl[0].pool[kop.W] = 1
	if !s.payExtortPip(0) {
		t.Fatal("the plain W did not pay")
	}
	if s.pl[0].pool[kop.W] != 0 || s.rb[0][0].n[kop.W] != 1 {
		t.Fatalf("pay took the wrong unit: plain W %d, restricted W %d", s.pl[0].pool[kop.W], s.rb[0][0].n[kop.W])
	}
}
