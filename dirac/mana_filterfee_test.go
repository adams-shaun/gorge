package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFiltFeeUnitSlotOrder: a {1} filter (Abzan Devotee: {1}: add {W}, {B} or
// {G}) beside a Swamp and a Plains pays {X}{B}{B}'s window the same whichever
// land sits first in slot order. The Plains pays the fee and the filter's B
// joins the Swamp's, so X reaches 0; scoring each fee unit before the filter's
// own output is added reads both units as -1 and takes the first group, which
// left no X at all when the Swamp came first (Darkwater Catacombs on a black X
// spell, Forest and Swamp).
func TestFiltFeeUnitSlotOrder(t *testing.T) {
	var cost kop.ManaCost
	cost.X = 1
	cost.Pips[2] = 2 // {X}{B}{B}
	for _, lands := range [][]string{{"Swamp", "Plains"}, {"Plains", "Swamp"}} {
		sc, dev, _, _ := devoteeScen(t, lands...)
		s, k := sc.s, sc.k
		sl := sc.slot(dev)
		if !s.scriptFilt(k, 0).has(sl) || s.filtFee(k, sl) != 1 || s.filtUnits(k, sl) != 1 {
			t.Fatalf("lands %v: the Devotee is no {1} -> one-mana filter gap", lands)
		}
		var av avail
		s.availFor(k, 0, &av)
		if mx := av.maxX(&cost, pipMaskOf(&cost)); mx != -1 {
			t.Fatalf("lands %v: plain maxX %d, want -1 (BB is not payable without the filter)", lands, mx)
		}
		var filt [maxFaceFilters]Slot
		filt[0] = sl
		w := av
		if !s.filtActivate(k, &w, &filt, 1, &cost, false) {
			t.Fatalf("lands %v: the filter's fee is not payable", lands)
		}
		if got := w.maxX(&cost, pipMaskOf(&cost)); got != 0 {
			t.Errorf("lands %v: maxX with the filter %d, want 0", lands, got)
		}
	}
}
