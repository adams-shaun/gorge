package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestControlKeywordRepeats: each live control record grants its AddKWs$
// keyword once, so two Involuntary Employments on the same creature list
// haste twice (FDN pair 60288). FeatGrantedRep unioned the control records'
// haste into one bit, so the second listing went missing from the feat
// encoding's sparse bag.
func TestControlKeywordRepeats(t *testing.T) {
	sc := newScen(t, Config{})
	o := sc.perm(v.Plains, 0)
	sl := sc.slot(o)
	for i := 0; i < 2; i++ {
		if !sc.s.gainControl(sl, 1, 1, kop.DurEOT, NoObj, 0, false) {
			t.Fatal("gainControl refused a control record")
		}
		// OpGainControlEOT / gainControlOne's AddKWs$ rider
		sc.s.g.fx[sc.s.g.nfx-1].r.fl |= fcHaste
	}
	if got := sc.s.FeatCtrlKw(o); got&kop.KwHaste == 0 {
		t.Fatalf("precondition: control records grant %#x, want Haste", got)
	}
	g, rep, _ := sc.s.FeatGrantedRep(o)
	if g&kop.KwHaste == 0 {
		t.Fatalf("precondition: granted %#x lacks Haste", g)
	}
	b := bits.TrailingZeros32(uint32(kop.KwHaste))
	if got := int(rep[b]) + 1; got != 2 {
		t.Fatalf("haste listed %d times, want 2 (one per control record)", got)
	}
}
