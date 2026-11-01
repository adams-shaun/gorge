package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHybColoursPrintedOrder: the helper reads HybOrd and Resolve shifts it,
// so a two-pip cost keeps each pip's own order.
func TestHybColoursPrintedOrder(t *testing.T) {
	c := kop.ManaCost{
		Hyb:    [len(kop.ManaCost{}.Hyb)]kop.ColorMask{kop.ColW | kop.ColG, kop.ColU | kop.ColG},
		HybOrd: 0b01,
	}
	if f, s := hybColours(c, 0); f != kop.G || s != kop.W {
		t.Fatalf("pip 0 printed order %d,%d, want G,W", f, s)
	}
	if f, s := hybColours(c, 1); f != kop.U || s != kop.G {
		t.Fatalf("pip 1 printed order %d,%d, want U,G", f, s)
	}
	r := c.Resolve(kop.G)
	if r.Hyb[0] != kop.ColU|kop.ColG || r.HybOrd != 0 {
		t.Fatalf("after resolving G: Hyb %v HybOrd %d, want the U/G pip in order", r.Hyb, r.HybOrd)
	}
	if f, s := hybColours(r, 0); f != kop.U || s != kop.G {
		t.Fatalf("shifted pip printed order %d,%d, want U,G", f, s)
	}
}

// TestHybWitnessPrintedOrder: the payment witness resolves a hybrid pip to
// the printed first colour, as gorge's potential planner walks
// AnnouncePip's {A},{B}. {G/W} pays G when both are available; {W/G} pays W.
func TestHybWitnessPrintedOrder(t *testing.T) {
	sc, id := newBLBScen(t, []cardset.Entry{
		{Name: "Brightblade Stoat", Count: 1},
		{Name: "Forest", Count: 1},
		{Name: "Plains", Count: 1},
	})
	sc.lands(0, id["Forest"], 1)
	sc.lands(0, id["Plains"], 1)
	for _, tc := range []struct {
		name string
		ord  uint8
		want int
	}{
		{"printed W/G", 0, kop.W},
		{"printed G/W", 1, kop.G},
	} {
		cost := kop.ManaCost{
			Hyb:    [len(kop.ManaCost{}.Hyb)]kop.ColorMask{kop.ColG | kop.ColW},
			HybOrd: tc.ord,
		}
		got := sc.s.hybResolve(sc.k, 0, cost, 0)
		if got.Pips[tc.want] != 1 {
			t.Fatalf("%s: witness %+v, want a %d pip", tc.name, got, tc.want)
		}
	}
}
