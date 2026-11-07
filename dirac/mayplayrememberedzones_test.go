package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMayPlayRememberedBothZones: Ark of Hunger's lowering emits one OpMayPlay
// per zone, so a remembered card in the graveyard and one in exile each get
// exactly one permission, and a card that changes zone loses it (its record's
// version goes stale).
func TestMayPlayRememberedBothZones(t *testing.T) {
	sc, _ := newBLBScen(t, []cardset.Entry{{Name: "Plains", Count: 8}})
	s := sc.s
	gy, ex := s.pl[0].lib[0], s.pl[0].lib[1]
	for _, m := range []struct {
		o Obj
		z kop.Zone
	}{{gy, kop.ZoneGraveyard}, {ex, kop.ZoneExile}} {
		s.detach(m.o)
		s.place(m.o, m.z, false)
	}
	s.g.rem[0], s.g.rem[1], s.g.nrem = gy, ex, 2

	// The two instructions the lowering emits, in graveyard-then-exile order.
	in := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefRemembered, C: int16(kop.ZoneGraveyard), A: kop.Const(kop.DurEOT)}
	s.mayPlayRemembered(sc.k, &in, 0)
	in.C = int16(kop.ZoneExile)
	s.mayPlayRemembered(sc.k, &in, 0)

	valid := func(o Obj) int {
		n := 0
		for i := 0; i < int(s.g.nmay); i++ {
			if r := s.g.may[i]; r.obj == o && r.ver == s.objs[o].ver {
				n++
			}
		}
		return n
	}
	if s.g.nmay != 2 || s.g.may[0].obj != gy || s.g.may[1].obj != ex {
		t.Fatalf("permissions %d (%v), want the graveyard card then the exiled one", s.g.nmay, s.g.may[:s.g.nmay])
	}
	if a, b := valid(gy), valid(ex); a != 1 || b != 1 {
		t.Fatalf("valid permissions %d/%d, want one for each remembered card", a, b)
	}

	// The graveyard card moves on: its record's version no longer matches.
	s.detach(gy)
	s.place(gy, kop.ZoneExile, false)
	if n := valid(gy); n != 0 {
		t.Fatalf("the moved card kept %d permissions", n)
	}
	if n := valid(ex); n != 1 {
		t.Fatalf("the unmoved card has %d permissions, want one", n)
	}
}
