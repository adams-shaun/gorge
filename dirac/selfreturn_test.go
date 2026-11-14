package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfReturnFindsNoNewObject: Reassembling Skeleton's "{1}{B}: Return
// this card from your graveyard to the battlefield" does nothing once the
// card has left the graveyard in response (Abyssal Harvester exiled it):
// gorge reads the zone (the exiled card is a new object, CR 400.7). Before,
// the kernel returned
// it from exile (pairs 3630 7853).
func TestSelfReturnFindsNoNewObject(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Reassembling Skeleton", Count: 1}, {Name: "Swamp", Count: 5}})
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Swamp"], 0)
	sk := sc.obj(id["Reassembling Skeleton"], 0)
	sc.s.place(sk, kop.ZoneGraveyard, false)
	sc.do(t, AkActivate, TCode(sk), scAny)
	sc.stepTo(t, 6, func() bool { return sc.s.sp > 0 && sc.s.ctl == ctlPriority }, nil)
	if sc.s.sp == 0 {
		t.Fatal("the ability is not on the stack")
	}
	sc.s.detach(sk)
	sc.s.place(sk, kop.ZoneExile, false) // exiled in response
	sc.stepTo(t, 6, func() bool { return sc.s.sp == 0 }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if z := sc.s.objs[sk].zone; z != kop.ZoneExile {
		t.Fatalf("Skeleton zone %d, want it left in exile", z)
	}
}

// TestSkeletonReturnSameObject: two activations of Reassembling Skeleton's
// "return this card from your graveyard": the first returns it, it dies
// again, and the second finds a new object in the graveyard and returns
// nothing (CR 400.7; gorge's incarnation stamp, the 13 slice
// pairs, 76 of the 10k). Before, the kernel checked the zone only.
func TestSkeletonReturnSameObject(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Reassembling Skeleton", Count: 1}, {Name: "Swamp", Count: 8}})
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	sk := sc.obj(id["Reassembling Skeleton"], 0)
	sc.s.place(sk, kop.ZoneGraveyard, false)
	acts := 0
	pick := func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; acts < 2 && a.Kind() == AkActivate && a.Source() == TCode(uint8(sk)) {
				acts++
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	}
	sc.stepTo(t, 20, func() bool { return sc.s.objs[sk].zone == kop.ZoneBattlefield }, pick)
	if acts != 2 || sc.s.objs[sk].zone != kop.ZoneBattlefield || sc.s.sp != 1 {
		t.Fatalf("activations %d, Skeleton zone %d, stack %d: want two, the battlefield and one left", acts, sc.s.objs[sk].zone, sc.s.sp)
	}
	sc.s.leave(sc.k, sc.slot(sk), kop.ZoneGraveyard, false) // it dies again
	sc.stepTo(t, 20, func() bool { return sc.s.sp == 0 }, pick)
	if z := sc.s.objs[sk].zone; z != kop.ZoneGraveyard {
		t.Fatalf("Skeleton zone %d, want it left in the graveyard (a new object)", z)
	}
}
