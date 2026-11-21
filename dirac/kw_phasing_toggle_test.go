package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPhasingKeywordToggles pins the untap step's keyword half of phasing
// (CR 702.26a, gorge finishUntapStep): a phased-in carrier of the printed
// keyword phases out as the step begins (its sickness ended with the turn
// change), a phased-out one phases in at the next, a plain creature and a
// carrier that lost all abilities are left alone, and another seat's untap
// step touches none of them.
func TestPhasingKeywordToggles(t *testing.T) {
	set := cardsettest.Set(t, "phasing")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the pool", name)
		}
		return CardID(set.PoolID(i))
	}
	drake, merfolk := id("Teferi's Drake"), id("Merfolk of the Pearl Trident")
	sc := newScenTable(t, &tbl, Config{})
	s, k := sc.s, sc.k
	if !k.t.Cards[drake].Phasing || k.t.Cards[merfolk].Phasing || !k.phasing {
		t.Fatal("Teferi's Drake must print phasing (and switch the table's phasing on), the Merfolk not")
	}
	car, plain, blank := sc.perm(drake, 0), sc.perm(merfolk, 0), sc.perm(drake, 0)
	s.perms[sc.slot(blank)].ovr.flags |= ovLoseAll
	s.perms[sc.slot(car)].flags |= pfSick
	s.active = 1
	s.phaseUntap(k)
	if s.hiddenAt(sc.slot(car)) {
		t.Fatal("the other seat's untap step phased a carrier out")
	}
	s.active = 0
	s.phaseUntap(k)
	if !s.hiddenAt(sc.slot(car)) || s.perms[sc.slot(car)].flags&pfSick != 0 {
		t.Fatal("a phased-in carrier must phase out at its controller's untap step, its sickness ended")
	}
	if s.hiddenAt(sc.slot(plain)) || s.hiddenAt(sc.slot(blank)) {
		t.Fatal("a plain creature and a carrier that lost its abilities stay in")
	}
	s.phaseUntap(k)
	if s.hiddenAt(sc.slot(car)) || !s.occ.has(sc.slot(car)) || s.objs[car].zone != kop.ZoneBattlefield {
		t.Fatal("a phased-out carrier must phase in at the next untap step, and not phase out again that step")
	}
}
