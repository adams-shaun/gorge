package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttachedRefLKI: "equipped creature" read after the Equipment left
// the battlefield is the creature it was attached to as it left (CR
// 608.2h; Adventuring Gear destroyed in response to its landfall trigger,
// pairs 15721 47715 85781). Before, the ref named nothing.
func TestAttachedRefLKI(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Adventuring Gear", Count: 1}, {Name: "Fiendish Panda", Count: 1}, {Name: "Forest", Count: 8}})
	g := sc.perm(id["Adventuring Gear"], 0)
	pd := sc.perm(id["Fiendish Panda"], 0)
	sc.s.perms[sc.slot(g)].attached = sc.slot(pd)
	slot := sc.slot(g)
	sc.s.leave(sc.k, slot, kop.ZoneGraveyard, false)
	// Reuse the departed Equipment's slot before the ability resolves: the
	// live slot scan cannot find it any more (Spiral into Solitude
	// sacrificing itself, a flash creature entering in response).
	np := sc.perm(id["Fiendish Panda"], 0)
	if sc.slot(np) != slot {
		t.Fatalf("new Panda at slot %d, want the Gear's %d", sc.slot(np), slot)
	}
	if got := sc.s.lkiAttached(g); got != pd {
		t.Fatalf("lkiAttached = %d, want the Panda %d", got, pd)
	}
	sc.s.res.e = stackEntry{obj: g}
	if got := sc.s.objRef(sc.k, kop.RefAttached); got == NoSlot || sc.s.perms[got].obj != pd {
		t.Fatalf("RefAttached of the departed Gear = slot %d, want the Panda's", got)
	}
}
