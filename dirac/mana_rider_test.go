package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestManaAbilityRider: Twitching Doll's "{T}: Add one mana of any color. Put
// a nest counter on this" is a source gorge's planner defers (its rider,
// "source:rider"): a {G} payment taps the Forest that entered after it, not
// the Doll first in battlefield order; with no other plan the second pass taps
// the Doll (the potential play's prefix), and the tap puts its nest counter on
// it (CR 605.3b).
func TestManaAbilityRider(t *testing.T) {
	deck := []cardset.Entry{{Name: "Twitching Doll", Count: 1}, {Name: "Forest", Count: 19}}
	tbl, da, _, err := cardsettest.Synth(t, "std-dsk").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	doll, forest := id["Twitching Doll"], id["Forest"]
	if _, _, ok := cardManaRider(&tbl.Cards[doll]); !ok {
		t.Fatal("precondition: Twitching Doll's mana ability is not the counter-rider shape")
	}
	nest := int(kop.CtrNest)
	run := func(t *testing.T, withForest bool, col int) (dollTapped, forestTapped bool, ctr int) {
		sc := newScenTableLib(t, tbl, forest, Config{NoElide: true})
		if !sc.k.gen.anyManaRider {
			t.Fatal("precondition: the table has no counter-rider source")
		}
		do := sc.perm(doll, 0)
		var fo Obj
		if withForest {
			fo = sc.perm(forest, 0)
		}
		if !sc.s.planDeferred(sc.k, 0, sc.slot(do)) || !sc.s.scriptTapsDefer(sc.k, 0).has(sc.slot(do)) {
			t.Fatal("precondition: the Doll is not a deferred source and prefix gap")
		}
		if n := sc.s.perms[sc.slot(do)].ctrN(sc.s, nest); n != 0 {
			t.Fatalf("precondition: the Doll entered with %d nest counters", n)
		}
		c := cost(0, col)
		sc.s.tapFor(sc.k, 0, &c, 0, NoObj)
		return sc.tapped(do), withForest && sc.tapped(fo), sc.s.perms[sc.slot(do)].ctrN(sc.s, nest)
	}
	t.Run("an ordinary source pays", func(t *testing.T) {
		if dt, ft, n := run(t, true, kop.G); dt || !ft || n != 0 {
			t.Fatalf("Doll tapped %v, Forest tapped %v, nest counters %d: want the Forest alone, no counter", dt, ft, n)
		}
	})
	t.Run("no other plan taps it and its rider puts the counter", func(t *testing.T) {
		if dt, _, n := run(t, false, kop.R); !dt || n != 1 {
			t.Fatalf("Doll tapped %v, nest counters %d: want it tapped with one nest counter", dt, n)
		}
	})
}
