package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantOnlySourceExtrasKeepTheBoundOpen: a creature whose only mana
// abilities are granted (Citanul Hierophants' "{T}: Add {G}", then Cryptolith
// Rite's any-colour one) reads the first as its view, so the potential walk
// admits the second as an extra. In the bounded mode that extra is one wildcard
// unit, and it must not close a bound an earlier source already opened.
func TestGrantOnlySourceExtrasKeepTheBoundOpen(t *testing.T) {
	sc, _, objs := tokScen(t, Config{NoElide: true}, "Citanul Hierophants", "Cryptolith Rite", "Grizzly Bears")
	s := sc.s
	sl := sc.slot(objs[2])
	var gi [maxGrants]int16
	if n := s.grantIdxs(sc.k, sl, &gi); n != 2 || s.ownManaUsable(sc.k, sl) {
		t.Fatalf("precondition: %d grants (own usable %v), want two grants and no own ability", n, s.ownManaUsable(sc.k, sl))
	}
	first, second := s.grantView(sc.k.gen, gi[0], sl), s.grantView(sc.k.gen, gi[1], sl)
	if first.manaCol == kop.ColAny || second.manaCol != kop.ColAny {
		t.Fatalf("precondition: grant colours %x, %x, want the fixed one first and the any-colour one second", first.manaCol, second.manaCol)
	}
	for _, wasOpen := range []bool{false, true} {
		cen := potCensus{on: true}
		cen.recTap(sl, s.srcMaskAt(first, sl), first.manaN)
		open, anyFree, folded := wasOpen, 0, 0
		add := func(Slot, uint8, uint8) { t.Fatal("the extra tap grant must widen the source's tap unit, not add one") }
		fold := func(_ Slot, _ uint8, n uint8) { folded += int(n) }
		s.potentialGrantedExtra(sc.k, sl, nil, true, add, fold, &cen, &open, &anyFree)
		if anyFree != 1 || folded != 1 {
			t.Errorf("open %v: wildcard units %d, folded %d, want the second grant counted once", wasOpen, anyFree, folded)
		}
		if open != wasOpen {
			t.Errorf("bound open %v after the extras, want %v kept", open, wasOpen)
		}
	}
}
