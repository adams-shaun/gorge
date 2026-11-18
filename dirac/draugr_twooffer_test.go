package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoDraugrOfferOneCast: two Draugr Necromancers each grant the same
// permission over an ICE-countered exiled card; gorge folds the identical
// grants into one offer, so the kernel lists one cast of the card.
func TestTwoDraugrOfferOneCast(t *testing.T) {
	sc, id := exileCtrScen(t)
	s, k := sc.s, sc.k
	sc.perm(id["Draugr Necromancer"], 0)
	sc.perm(id["Draugr Necromancer"], 0)
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 0)
	}
	bear := sc.perm(id["Grizzly Bears"], 1)
	// two Draugr's exile-instead replacements compete: the player is asked
	// their order (CR 616.1), and the answered one stamps its counter
	s.leaveParked(k, sc.slot(bear))
	if !s.parkedWaiting() {
		t.Fatal("precondition: the death is not parked for its order ask")
	}
	var d Decision
	s.exileOrderOptions(k, &d)
	s.applyExileOrder(k, d.Options[0])
	if z := s.objs[bear].zone; z != kop.ZoneExile {
		t.Fatalf("precondition: bear is in zone %d, want exile", z)
	}
	if n := s.exileCounters(bear, kop.CtrIce); n != 1 {
		t.Fatalf("precondition: bear has %d ICE counters, want 1", n)
	}
	if casts := sc.castOptions(bear); len(casts) != 1 {
		t.Fatalf("two Draugr offer %d casts of the exiled bear (%v), want 1", len(casts), casts)
	}
}
