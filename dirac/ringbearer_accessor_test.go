package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRingBearerAccessorReadsLive: RingBearer is the exported live read, so a
// designation that a blink has ended is NoObj there too, not the stale record.
func TestRingBearerAccessorReadsLive(t *testing.T) {
	sc, id := newRingBearerScen(t)
	a := sc.perm(id["Grizzly Bears"], 0)
	sc.s.ringFold(sc.k, 0, a)
	if got := sc.s.RingBearer(0); got != a {
		t.Fatalf("RingBearer before the blink = %d, want the designated Bear %d", got, a)
	}
	sl := sc.slot(a)
	sc.s.leave(sc.k, sl, kop.ZoneExile, false)
	sc.s.enter(sc.k, a, 0, false, kop.ZoneExile)
	if got := sc.s.RingBearer(0); got != NoObj {
		t.Fatalf("RingBearer after the blink = %d, want NoObj (the designation ended)", got)
	}
}
