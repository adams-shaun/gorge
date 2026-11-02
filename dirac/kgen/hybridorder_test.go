package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHybridOutOfOrderLowering: a pair printed outside WUBRG order is no
// longer refused; the mask is the same and HybOrd records that gorge's
// AnnouncePip offers the printed first colour first.
func TestHybridOutOfOrderLowering(t *testing.T) {
	for _, tc := range []struct {
		sym   string
		mask  kop.ColorMask
		ord   uint8
		first int
	}{
		{"WU", kop.ColW | kop.ColU, 0, kop.W},
		{"GW", kop.ColW | kop.ColG, 1, kop.G},
		{"RW", kop.ColW | kop.ColR, 1, kop.R},
		{"GU", kop.ColU | kop.ColG, 1, kop.G},
		{"BG", kop.ColB | kop.ColG, 0, kop.B},
	} {
		got, err := manaCostOf(manaCost{Hybrid: []string{tc.sym}})
		if err != nil {
			t.Fatalf("%s: %v", tc.sym, err)
		}
		if got.Hyb[0] != tc.mask || got.Hyb[1] != 0 || got.HybOrd != tc.ord {
			t.Fatalf("%s: Hyb %v HybOrd %d, want mask %v order %d", tc.sym, got.Hyb, got.HybOrd, tc.mask, tc.ord)
		}
		r := got.Resolve(tc.first)
		if r.Pips[tc.first] != 1 || r.Hyb[0] != 0 || r.HybOrd != 0 {
			t.Fatalf("%s: resolving %d gave %+v", tc.sym, tc.first, r)
		}
	}
}

// TestHybridSecondPipOrder: the two pips keep independent order bits, and
// Resolve shifts them with the pips.
func TestHybridSecondPipOrder(t *testing.T) {
	got, err := manaCostOf(manaCost{Hybrid: []string{"GW", "WU"}})
	if err != nil {
		t.Fatal(err)
	}
	if got.HybOrd != 1 {
		t.Fatalf("HybOrd %d, want 1 (only the first pip reversed)", got.HybOrd)
	}
	r := got.Resolve(kop.G)
	if r.Hyb[0] != kop.ColW|kop.ColU || r.HybOrd != 0 {
		t.Fatalf("after G: Hyb %v HybOrd %d, want the W/U pip in order", r.Hyb, r.HybOrd)
	}
}
