package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMayPlayRemembered: OpMayPlay over the remembered set grants every
// remembered card still in the zone (gorge's Effect holds the whole list);
// with the ExileOnMoved$ lifetime (B=1) more than one remembered card stalls.
func TestMayPlayRemembered(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	s := sc.s
	a, b, c := s.pl[0].lib[0], s.pl[0].lib[1], s.pl[0].lib[2]
	for _, o := range []Obj{a, b} {
		s.detach(o)
		s.objs[o].zone = kop.ZoneExile
	}
	s.g.rem[0], s.g.rem[1], s.g.rem[2], s.g.nrem = a, b, c, 3 // c stays in the library
	in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefRemembered, C: int16(kop.ZoneExile), A: kop.Const(kop.DurEOT)}
	s.mayPlayRemembered(sc.k, &in, 0)
	if s.g.nmay != 2 || s.g.may[0].obj != a || s.g.may[1].obj != b {
		t.Fatalf("permissions %d (%v), want the two exiled remembered cards", s.g.nmay, s.g.may[:s.g.nmay])
	}
	s.g.nmay = 0
	in.B = kop.Const(kop.MayPlayNonLand) // the remembered Plains are lands
	s.mayPlayRemembered(sc.k, &in, 0)
	if s.g.nmay != 0 {
		t.Fatalf("a nonland grant gave %d lands a permission", s.g.nmay)
	}
	s.g.on = true
	in.B = kop.Const(1)
	s.mayPlayRemembered(sc.k, &in, 0)
	if s.g.nmay != 0 || s.g.stalled != StallExileOnMoved {
		t.Fatalf("ExileOnMoved over two remembered cards: %d permissions, stall %d", s.g.nmay, s.g.stalled)
	}
}
