package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// lethalPitScen is openCensusScen's grant pose (no Gemhide) plus a Cabal Pit
// (a fixed-{B} pain-only source: {T}: Add {B}, it deals 2 damage to you),
// tapped or not, with p0 at 1 life. The Pit's tap would end its controller,
// so gorge's planner never takes it (CR 119.4): an untapped Pit adds no plan
// and takes none away.
func lethalPitScen(t *testing.T, nSwamps, nCtr, nTap int, pitTapped bool) (*scen, Obj) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Rain of Filth", Count: 1},
		{Name: "Saprazzan Skerry", Count: 1},
		{Name: "Caligo Skin-Witch", Count: 1},
		{Name: "Island", Count: 1},
		{Name: "Swamp", Count: nSwamps},
		{Name: "Cabal Pit", Count: 1},
		{Name: "Plains", Count: 8},
	}
	sc, id := newSetScenAutopay(t, deck)
	skerry := sc.perm(id["Saprazzan Skerry"], 0)
	ssl := sc.slot(skerry)
	a, ok := sc.s.riderAbilityOf(sc.k, ssl)
	if !ok {
		t.Fatal("precondition: Saprazzan Skerry is not the bounded-rider shape")
	}
	pm := &sc.s.perms[ssl]
	pm.subCtrN(sc.s, int(a.ManaRiderCtr), pm.ctrN(sc.s, int(a.ManaRiderCtr)))
	if nCtr > 0 && !pm.addCtrN(sc.s, int(a.ManaRiderCtr), nCtr) {
		t.Fatal("precondition: could not put the depletion counters on")
	}
	pm.flags &^= pfTapped
	var tappable []Obj
	for i := 0; i < nSwamps; i++ {
		tappable = append(tappable, sc.perm(id["Swamp"], 0))
	}
	tappable = append(tappable, sc.perm(id["Island"], 0))
	caligo := sc.hand(id["Caligo Skin-Witch"], 0)
	rof := sc.hand(id["Rain of Filth"], 0)
	sc.do(t, AkCast, TCode(rof), scAny)
	sc.resolveAll(t)
	for _, o := range tappable {
		if nTap == 0 {
			break
		}
		if sc.tapped(o) {
			continue // this one paid the Rain's {B}
		}
		sc.s.perms[sc.slot(o)].flags |= pfTapped
		nTap--
	}
	if nTap > 0 {
		t.Fatal("precondition: not enough untapped lands to tap")
	}
	pit := sc.perm(id["Cabal Pit"], 0)
	psl := sc.slot(pit)
	if pc := sc.s.manaPCAt(sc.k, psl); !pc.painOnly || pc.painCol == 0 || pc.painN < 1 {
		t.Fatal("precondition: Cabal Pit is not a pain-only source")
	}
	if pitTapped {
		sc.s.perms[psl].flags |= pfTapped
	}
	sc.s.pl[0].life = 1
	return sc, caligo
}

// TestKickedLethalPainSourceNoDoubleRemoval: a lethal fixed-colour pain-only
// source stays out of the kicked potential proof's per-tap census once
// (potentialTapUnit). An untapped Cabal Pit at 1 life must offer the kicked
// Caligo Skin-Witch exactly where the tapped Pit does: removing its {B} a
// second time takes a Swamp's or a Rain grant's {B} instead.
func TestKickedLethalPainSourceNoDoubleRemoval(t *testing.T) {
	for _, pose := range [][3]int{{1, 1, 0}, {2, 1, 2}} {
		sc, c := lethalPitScen(t, pose[0], pose[1], pose[2], true)
		if !offeredKickedCast(sc, c) {
			t.Fatalf("precondition: pose %v with the Pit tapped does not offer the kicked cast", pose)
		}
		sc, c = lethalPitScen(t, pose[0], pose[1], pose[2], false)
		if !offeredKickedCast(sc, c) {
			t.Errorf("pose %v: an untapped Cabal Pit at 1 life withholds the kicked cast the tapped Pit offers", pose)
		}
	}
}
