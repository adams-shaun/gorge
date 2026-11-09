package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDigImprintMarker: a take-all exile dig followed by the DigImprint marker
// imprints every card it moves (not the remembered set), and a MayPlay over
// RefImprinted grants each of them; the same dig without the marker imprints
// nothing, and the clear op empties the set.
func TestDigImprintMarker(t *testing.T) {
	dig := kop.Instr{Op: kop.OpDigEx, Obj: kop.RefController, A: kop.Const(3), B: kop.Const(-1),
		C: int16(kop.ZoneExile) | int16(kop.ZoneLibrary)<<3}
	mark := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtDigImprint))}
	play := kop.Instr{Op: kop.OpMayPlay, Obj: kop.RefImprinted, A: kop.Const(0), C: int16(kop.ZoneExile)}
	clear := kop.Instr{Op: kop.OpExt, A: kop.Const(int(kop.ExtClearImprinted))}

	sc, _, _ := imprintScen(t, []kop.Instr{dig, mark, play})
	s := sc.s
	pl := &s.pl[0]
	top := append([]Obj(nil), pl.lib[pl.libLo:pl.libLo+3]...)
	s.runProg(sc.k)
	if s.g.nimp != 3 {
		t.Fatalf("imprinted %d objects, want the 3 the dig exiled", s.g.nimp)
	}
	for i, o := range top {
		if s.g.imp[i] != o || s.objs[o].zone != kop.ZoneExile {
			t.Fatalf("imprinted[%d] = %d (zone %d), want exiled top card %d", i, s.g.imp[i], s.objs[s.g.imp[i]].zone, o)
		}
	}
	if s.g.nrem != 0 {
		t.Fatalf("the dig remembered %d objects, want none", s.g.nrem)
	}
	if s.g.nmay != 3 {
		t.Fatalf("%d play permissions, want one per imprinted card", s.g.nmay)
	}

	sc, _, _ = imprintScen(t, []kop.Instr{dig, play})
	sc.s.runProg(sc.k)
	if sc.s.g.nimp != 0 || sc.s.g.nmay != 0 {
		t.Fatalf("without the marker: imprinted %d, permissions %d, want none", sc.s.g.nimp, sc.s.g.nmay)
	}

	sc, _, _ = imprintScen(t, []kop.Instr{dig, mark, clear})
	sc.s.runProg(sc.k)
	if sc.s.g.nimp != 0 {
		t.Fatalf("after the clear: imprinted %d, want none", sc.s.g.nimp)
	}
}
