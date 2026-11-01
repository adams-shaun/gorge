package dirac

import (
	"math/bits"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestLoseAllRemovesOlderGrants: "loses all abilities" (Witness
// Protection's Aura static) applies in layer 6 in timestamp order with the
// keyword grants (CR 613.7, gorge grantLostTo): an Equipment's flying
// grant older than the Aura is removed with the rest, a newer one stays.
// Pair 3168 of the 10k slice: a Celestial Armor-equipped creature under a
// later Witness Protection kept flying in the kernel, so Ambush Wolf was
// not offered as a blocker.
func TestLoseAllRemovesOlderGrants(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Celestial Armor", Count: 1},
		{Name: "Ambush Wolf", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	for _, armorFirst := range []bool{true, false} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		wolf := sc.perm(id["Ambush Wolf"], 0)
		attach := func(name string) {
			o := sc.perm(id[name], 0)
			sc.s.perms[sc.slot(o)].attached = sc.slot(wolf)
		}
		if armorFirst {
			attach("Celestial Armor")
			attach("Witness Protection")
		} else {
			attach("Witness Protection")
			attach("Celestial Armor")
		}
		m, _ := sc.s.FeatKeywords(wolf)
		if got, want := m&kop.KwFlying != 0, !armorFirst; got != want {
			t.Errorf("armor attached first=%v: flying %v, want %v (keywords %#x)", armorFirst, got, want, m)
		}
		g, _ := sc.s.FeatGranted(wolf)
		if got, want := g&kop.KwFlying != 0, !armorFirst; got != want {
			t.Errorf("armor attached first=%v: granted flying %v, want %v", armorFirst, got, want)
		}
	}
}

// TestEOTKeywordRepeatsPastThree: gorge lists an until-end-of-turn keyword
// once per granting effect, so a fourth and fifth grant (Balmor,
// Battlemage Captain's trample from four spells, pair 4599) are listed
// too; perm.eotKwRep held only three.
func TestEOTKeywordRepeatsPastThree(t *testing.T) {
	sc := newScen(t, Config{})
	o := sc.perm(v.Plains, 0)
	sl := sc.slot(o)
	for i := 0; i < 5; i++ {
		sc.s.grantEOT(sl, kop.KwTrample)
	}
	_, rep, _ := sc.s.FeatGrantedRep(o)
	if n := rep[bits.TrailingZeros32(uint32(kop.KwTrample))]; n != 4 {
		t.Fatalf("trample listed %d times, want 5", n+1)
	}
	// the repeats end with the turn
	sc.s.finishCleanup(sc.k)
	if g, rep, _ := sc.s.FeatGrantedRep(o); g != 0 || rep != (FeatKwReps{}) || sc.s.nkwx != 0 {
		t.Fatalf("after cleanup: granted %#x rep %v, %d repeat records", g, rep, sc.s.nkwx)
	}
}

// TestCleanupMarksSuppressedEOTGrants: a suppressed until-end-of-turn grant
// (a kwSup record left by suppressEOT) is part of the permanent's view
// (ObjHash and FeatGrantedRep read it), so finishCleanup must mark the slot
// before dropping the records. Otherwise a per-object view cache built while
// the grant was suppressed stays valid after cleanup and keeps a keyword the
// permanent no longer has.
func TestCleanupMarksSuppressedEOTGrants(t *testing.T) {
	sc := newScen(t, Config{})
	o := sc.perm(v.Plains, 0)
	sl := sc.slot(o)
	sc.s.grantEOT(sl, kop.KwTrample)
	sc.s.suppressEOT(sl)
	if g, _ := sc.s.FeatGranted(o); g&kop.KwTrample != 0 {
		t.Fatal("suppressEOT kept the trample grant visible")
	}
	sc.s.SetEpoch(sc.s.Epoch() + 1) // clear the marks the setup left
	at := sc.s.Touches()
	sc.s.finishCleanup(sc.k)
	if g, _ := sc.s.FeatGranted(o); g&kop.KwTrample != 0 {
		t.Fatalf("cleanup kept the suppressed grant: %v", g)
	}
	if !sc.s.Changed(o) || !sc.s.ChangedSince(o, at) {
		t.Fatal("finishCleanup ended the suppressed grant without marking the card changed: a cached card view stays stale")
	}
}

// TestLoseAllKeepsCantBeBlocked: Rogue's Passage's "target creature can't
// be blocked this turn" is an effect (gorge: an Effect object's CantBlockBy
// static naming the creature), not an ability the creature gains, so a
// later Witness Protection's loss of all abilities leaves it unblockable,
// while an until-end-of-turn flying grant is removed (pair 30601: p1's
// Dazzling Angel under Witness Protection was still unblockable in gorge).
func TestLoseAllKeepsCantBeBlocked(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Ambush Wolf", Count: 1}, {Name: "Plains", Count: 8}})
	wolf := sc.perm(id["Ambush Wolf"], 0)
	sl := sc.slot(wolf)
	sc.s.grantEOT(sl, kop.KwUnblockable|kop.KwFlying)
	wp := sc.perm(id["Witness Protection"], 0)
	sc.s.perms[sc.slot(wp)].attached = sl
	sc.s.loseAllOnset(sc.k, sc.slot(wp), sl)
	m, _ := sc.s.FeatKeywords(wolf)
	if m&kop.KwUnblockable == 0 || m&kop.KwFlying != 0 {
		t.Fatalf("keywords %#x: want can't be blocked kept and flying removed", m)
	}
}
