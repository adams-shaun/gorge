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

// TestUnattachedDepartureDropsLastBearer: an Aura or Equipment that left
// attached, returned as a new object, and left again while attached to
// nothing names no bearer (gorge Object.LastBearer = 0, CR 400.7 and
// 608.2h), not the creature an earlier incarnation was attached to.
func TestUnattachedDepartureDropsLastBearer(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{
		{Name: "Adventuring Gear", Count: 1},
		{Name: "Fiendish Panda", Count: 1},
		{Name: "Forest", Count: 8},
	})
	gear := sc.perm(id["Adventuring Gear"], 0)
	bearer := sc.perm(id["Fiendish Panda"], 0)
	first := sc.slot(gear)
	sc.s.perms[first].attached = sc.slot(bearer)
	sc.s.leave(sc.k, first, kop.ZoneGraveyard, false)
	if h, ok := sc.s.attLKIHost(gear); !ok || h != bearer {
		t.Fatalf("after the attached departure attLKIHost = (%d,%v), want (%d,true)", h, ok, bearer)
	}
	if h := sc.s.lastBearerObj(gear); h != bearer {
		t.Fatalf("after the attached departure lastBearerObj = %d, want %d", h, bearer)
	}
	sc.s.detach(gear)
	sc.s.enter(sc.k, gear, 0, false, kop.ZoneGraveyard)
	if sl := sc.slot(gear); sl != first {
		t.Fatalf("re-entered at slot %d, want the departed %d (the test isolates the attLKI record)", sl, first)
	}
	p := &sc.s.perms[first]
	if p.attached != NoSlot || p.flags&pfLastBear != 0 {
		t.Fatalf("re-entered attached=%d pfLastBear=%v, want unattached with no parked host", p.attached, p.flags&pfLastBear != 0)
	}
	if sc.s.objs[bearer].zone != kop.ZoneBattlefield {
		t.Fatalf("the old bearer is in zone %d; a stale record could not resolve", sc.s.objs[bearer].zone)
	}
	sc.s.leave(sc.k, first, kop.ZoneGraveyard, false)
	if h, ok := sc.s.attLKIHost(gear); ok || h != NoObj {
		t.Fatalf("after the unattached departure attLKIHost = (%d,%v), want (NoObj,false)", h, ok)
	}
	if h := sc.s.lastBearerObj(gear); h != NoObj {
		t.Fatalf("after the unattached departure lastBearerObj = %d, want NoObj", h)
	}
}
