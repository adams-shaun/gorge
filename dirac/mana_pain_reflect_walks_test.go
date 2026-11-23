package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The walks that price a painland's tap beside a reflect carrier (the filter
// search's step, the potential bound, the trigger X supply) read the same
// srcExtraPain the census does (mana_pain_reflect_pick_test.go): a painful
// source is no plain tap srcExtra can price, so none of them stalls on it.

// TestPainReflectedFilterStepPricesPick: a filter search's step that taps the
// painland for the pick it knows floats that pick's reflected extra, and a {C}
// tap its normal ability's ({C}{C}); no capacity stall.
func TestPainReflectedFilterStepPricesPick(t *testing.T) {
	for _, tc := range []struct {
		col  int
		name string
	}{{kop.W, "W"}, {kop.U, "U"}, {kop.C, "C"}} {
		sc, sl := painReflectScen(t)
		pool := &sc.s.pl[0].pool
		sc.s.filtDo(sc.k, 0, filtStep{sl: sl, ans: uint8(tc.col)})
		if sc.s.Stalled() {
			t.Fatalf("%s: the filter step stalled: %s", tc.name, StallName(sc.s.StallReason()))
		}
		for c, n := range pool {
			want := uint8(0)
			if c == tc.col {
				want = 2
			}
			if n != want {
				t.Errorf("%s: pool[%d] = %d, want %d (one tap of one type)", tc.name, c, n, want)
			}
		}
	}
}

// TestPainReflectedPotentialExtra: the potential bound folds the painland's
// normal {C} ability's extra (one more {C}) and does not stall.
func TestPainReflectedPotentialExtra(t *testing.T) {
	sc, sl := painReflectScen(t)
	pc := sc.s.manaPCGen(sc.k, sl)
	var folded [kop.NumManaKinds]int
	fold := func(_ Slot, mask, n uint8) {
		for c := range folded {
			if mask&(1<<c) != 0 {
				folded[c] += int(n)
			}
		}
	}
	sc.s.potentialTapExtra(sc.k, sl, pc, srcMask(pc), fold, &potCensus{})
	if sc.s.Stalled() {
		t.Fatalf("the potential walk stalled: %s", StallName(sc.s.StallReason()))
	}
	if folded[kop.C] != 1 {
		t.Errorf("folded %v, want one {C}", folded)
	}
}

// TestPainReflectedTrigXSupply: the trigger X supply counts the painland once,
// at its larger alternative's extra (one mana), with no stall.
func TestPainReflectedTrigXSupply(t *testing.T) {
	sc, _ := painReflectScen(t)
	if n := sc.s.trigXTapExtras(sc.k, 0); n != 1 {
		t.Errorf("trigXTapExtras = %d, want 1", n)
	}
	if sc.s.Stalled() {
		t.Fatalf("the supply stalled: %s", StallName(sc.s.StallReason()))
	}
}
