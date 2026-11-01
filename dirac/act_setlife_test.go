package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSetLife: OpSetLife (gorge effSetLife) moves each player's life to the
// amount as a loss or a gain of the difference (life lost / gained this
// turn count it).
func TestSetLife(t *testing.T) {
	sc, _ := newObjScen(t, []cardset.Entry{{Name: "Plains", Count: 4}})
	s := sc.s
	s.res.e.ctrl = 0
	s.exec(sc.k, &kop.Instr{Op: kop.OpSetLife, Obj: kop.RefController, A: kop.Const(7)})
	if s.pl[0].life != 7 || s.pl[0].lifeLost != 13 {
		t.Fatalf("life %d lost %d, want 7 and 13", s.pl[0].life, s.pl[0].lifeLost)
	}
	s.exec(sc.k, &kop.Instr{Op: kop.OpSetLife, Obj: kop.RefEachPlayer, A: kop.Const(9)})
	if s.pl[0].life != 9 || s.pl[0].lifeGained != 2 || s.pl[1].life != 9 || s.pl[1].lifeLost != 11 {
		t.Fatalf("each player: %d (+%d) and %d (-%d), want 9 (+2) and 9 (-11)", s.pl[0].life, s.pl[0].lifeGained, s.pl[1].life, s.pl[1].lifeLost)
	}
}
