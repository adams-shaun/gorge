package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEntrantsOfOneMoveSeeEachOther: Raise the Past returns Savannah
// Lions and Skyknight Squire together (graveyard order Lions first); the
// Squire's "whenever another creature you control enters" sees the Lions
// (CR 603.6a, gorge TestCR603_6aRaiseThePastEntrantsSeeEachOther).
// Before, the Lions entered before the Squire was there to see it.
func TestEntrantsOfOneMoveSeeEachOther(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Raise the Past", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Skyknight Squire", Count: 1}, {Name: "Plains", Count: 8}})
	sc.lands(0, id["Plains"], 4)
	lions := sc.grave(id["Savannah Lions"], 0, 1)[0]
	sq := sc.grave(id["Skyknight Squire"], 0, 1)[0]
	sc.castCard(t, sc.hand(id["Raise the Past"], 0))
	sc.resolveAll(t)
	if !sc.onBF(lions) || !sc.onBF(sq) {
		t.Fatal("Raise the Past did not return both creatures")
	}
	if n := sc.s.perms[sc.slot(sq)].ctrLow(int(kop.CtrP1P1)); n != 1 {
		t.Fatalf("Skyknight Squire +1/+1 counters %d, want 1", n)
	}
}

// TestFightHitsAreOneBatch: the fight's two hits are one damage event:
// each fighter is its hit's source, and a lifelink fighter gains once.
func TestFightHitsAreOneBatch(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Vampire Nighthawk", Count: 1}, {Name: "Savannah Lions", Count: 1}, {Name: "Swamp", Count: 8}})
	nh := sc.perm(id["Vampire Nighthawk"], 0)
	li := sc.perm(id["Savannah Lions"], 1)
	life := sc.s.pl[0].life
	var hb hitBatch
	hb.add(sc.s, sc.slot(nh), sc.slot(li), sc.s.power(sc.k, sc.slot(nh)))
	hb.add(sc.s, sc.slot(li), sc.slot(nh), sc.s.power(sc.k, sc.slot(li)))
	sc.s.dealHits(sc.k, hb.h[:hb.n])
	if d := sc.s.perms[sc.slot(li)].damage; d != 2 {
		t.Fatalf("Lions damage %d, want 2", d)
	}
	if d := sc.s.perms[sc.slot(nh)].damage; d != 2 {
		t.Fatalf("Nighthawk damage %d, want 2", d)
	}
	if g := sc.s.pl[0].life - life; g != 2 {
		t.Fatalf("lifelink gained %d, want 2", g)
	}
}
