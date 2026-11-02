package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDeathtouchMarksPlaneswalker: gorge marks every permanent a
// deathtouch source damages with its "Deathtouched" counter, a
// planeswalker too (effects/damage.go, rules/combat.go), and the net view
// lists it until the cleanup: Treetop Snarespinner biting Liliana (Bite
// Down; pairs 45578 47786 65911 67798 99143). Before, only a non-planeswalker
// recipient was marked.
func TestDeathtouchMarksPlaneswalker(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Vampire Nighthawk", Count: 1}, {Name: "Liliana, Dreadhorde General", Count: 1}, {Name: "Swamp", Count: 8}})
	nh := sc.perm(id["Vampire Nighthawk"], 0)
	li := sc.perm(id["Liliana, Dreadhorde General"], 1)
	sc.s.perms[sc.slot(li)].setCtrN(sc.s, int(kop.CtrLoyalty), 6)
	sc.s.damageBetween(sc.k, sc.slot(nh), sc.slot(li), 2)
	if l := sc.s.perms[sc.slot(li)].ctrLow(int(kop.CtrLoyalty)); l != 4 {
		t.Fatalf("Liliana's loyalty %d, want 4", l)
	}
	if m := sc.s.FeatDeathtouchMark(li); m != 1 {
		t.Fatalf("Liliana's deathtouch mark %d, want 1", m)
	}
}

// TestDeathtouchMarksCount: gorge adds a "Deathtouched" counter per
// deathtouch damage event, so a Gorehorn Raider blocked by two deathtouch
// creatures carries two (pair 90221). Before, the kernel kept one bit.
func TestDeathtouchMarksCount(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Vampire Nighthawk", Count: 2}, {Name: "Gorehorn Raider", Count: 1}, {Name: "Swamp", Count: 8}})
	a := sc.perm(id["Vampire Nighthawk"], 0)
	b := sc.perm(id["Vampire Nighthawk"], 0)
	g := sc.perm(id["Gorehorn Raider"], 1)
	var r dmgRec
	r.note(sc.slot(a), sc.slot(g), 1)
	r.note(sc.slot(b), sc.slot(g), 1)
	if n := sc.s.dtEvents(sc.k, &r, sc.slot(g)); n != 2 {
		t.Fatalf("deathtouch events %d, want 2", n)
	}
	sc.s.damageBetween(sc.k, sc.slot(a), sc.slot(g), 1)
	sc.s.damageBetween(sc.k, sc.slot(b), sc.slot(g), 1)
	if m := sc.s.FeatDeathtouchMark(g); m != 2 {
		t.Fatalf("Gorehorn Raider's deathtouch marks %d, want 2", m)
	}
}
