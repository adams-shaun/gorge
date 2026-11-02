package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSeek: OpSeek (gorge effSeek) samples the library from the top without
// replacement, one rng IntN(remaining) per card, and moves the picks to
// the hand; RememberFound$ (C&1) makes them the remembered set.
func TestSeek(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Valley Floodcaller", Count: 1}, {Name: "Plains", Count: 8}})
	fc := id["Valley Floodcaller"]
	a := sc.perm(fc, 0)
	sc.s.res.e.ctrl = 0
	sc.s.res.e.obj, sc.s.res.e.card = a, fc
	pl := &sc.s.pl[0]
	lib := append([]Obj(nil), pl.lib[pl.libLo:pl.libHi]...)
	if len(lib) < 3 {
		t.Fatalf("library of %d", len(lib))
	}
	want := sc.s.rng
	d0 := sc.s.rng.Draws
	hand0 := pl.handN
	in := kop.Instr{Op: kop.OpSeek, Obj: kop.RefController, A: kop.Const(2), C: 1}
	sc.s.exec(sc.k, &in)
	j1 := want.IntN(len(lib))
	p1 := lib[j1]
	rest := append(append([]Obj(nil), lib[:j1]...), lib[j1+1:]...)
	p2 := rest[want.IntN(len(rest))]
	if sc.s.rng.Draws != d0+2 || pl.handN != hand0+2 {
		t.Fatalf("draws %d, hand %d -> %d: want 2 draws and 2 more cards", sc.s.rng.Draws-d0, hand0, pl.handN)
	}
	for _, o := range []Obj{p1, p2} {
		if sc.s.objs[o].zone != kop.ZoneHand {
			t.Fatalf("picked card %d is in zone %d, not the hand", o, sc.s.objs[o].zone)
		}
	}
	if sc.s.g.nrem != 2 || sc.s.g.rem[0] != p1 || sc.s.g.rem[1] != p2 {
		t.Fatalf("remembered %v (n=%d), want [%d %d]", sc.s.g.rem[:sc.s.g.nrem], sc.s.g.nrem, p1, p2)
	}
}
