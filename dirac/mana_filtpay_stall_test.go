package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFiltPayStallReturns pins the payment probe's stall: a failed pay search
// that latched a rule gap must report it to the caller, not swallow it on the
// probe's fork. The board is the full restriction-kind table beside a
// first-only filter: the pay search activates the next restricted source,
// whose batch allocation latches StallRestrSrc, and the walk cannot pay the
// unless {20}.
func TestFiltPayStallReturns(t *testing.T) {
	sc, _ := filtRestrStallScen(t, kop.ColW|kop.ColU)
	cost := manaOf(20, 0)
	ok, stall := sc.s.filtPay(sc.k, 0, &cost, true)
	if ok {
		t.Fatal("the pay search priced an unless {20} twelve mana cannot make")
	}
	if stall != StallRestrSrc {
		t.Fatalf("the pay search swallowed its latched stall: got %s, want %s", StallName(stall), StallName(StallRestrSrc))
	}
	if sc.s.Stalled() {
		t.Fatalf("the probe declared its fork's stall itself: %s", StallName(sc.s.StallReason()))
	}
	// The window probe merges the pay search's stall into what it returns.
	if ok, st := sc.s.windowAffordsTake(sc.k, 0, &cost, true, true); ok || st != StallRestrSrc {
		t.Fatalf("the window probe returned ok=%v stall=%s, want the pay search's %s", ok, StallName(st), StallName(StallRestrSrc))
	}
	if sc.s.Stalled() {
		t.Fatalf("the window probe declared its fork's stall itself: %s", StallName(sc.s.StallReason()))
	}
}
