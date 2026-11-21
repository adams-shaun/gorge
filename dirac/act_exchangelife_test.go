package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExchangeLifeExt: ExtExchangeLife swaps the resolving controller's life
// with the named opponent's from one snapshot (gorge effExchangeLife's
// one-player arm), changes nothing on a tie, and the lockstep mutation applies
// only the controller's side.
func TestExchangeLifeExt(t *testing.T) {
	sc, _ := newObjScen(t, []cardset.Entry{{Name: "Savannah Lions", Count: 2}, {Name: "Island", Count: 4}})
	s := sc.s
	in := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtExchangeLife)), Obj: kop.RefOpponent}
	s.res.e.ctrl = 0

	s.pl[0].life, s.pl[1].life = 7, 15
	s.exec(sc.k, &in)
	if s.pl[0].life != 15 || s.pl[1].life != 7 {
		t.Fatalf("after the exchange life = %d/%d, want 15/7", s.pl[0].life, s.pl[1].life)
	}
	if s.pl[0].lifeGained != 8 || s.pl[1].lifeLost != 8 {
		t.Fatalf("tallies gained %d / lost %d, want 8 / 8 (each side is an ordinary life change)", s.pl[0].lifeGained, s.pl[1].lifeLost)
	}

	s.pl[0].life, s.pl[1].life = 12, 12
	s.exec(sc.k, &in)
	if s.pl[0].life != 12 || s.pl[1].life != 12 {
		t.Fatalf("a tie changed life to %d/%d", s.pl[0].life, s.pl[1].life)
	}

	s.pl[0].life, s.pl[1].life = 20, 3
	DebugExchangeLifeOneSide = true
	defer func() { DebugExchangeLifeOneSide = false }()
	s.exec(sc.k, &in)
	if s.pl[0].life != 3 || s.pl[1].life != 3 {
		t.Fatalf("DebugExchangeLifeOneSide life = %d/%d, want 3/3 (the controller's side only)", s.pl[0].life, s.pl[1].life)
	}
}
