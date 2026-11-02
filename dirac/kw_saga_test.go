package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSagaMultiCross: one lore-counter event that crosses more than one
// chapter queues one chapter trigger per chapter reached, in ascending order
// (CR 714.2b; gorge checkChapterTriggers). The constructed saga pool only
// has +1 lore sources (the proliferate spells), so the amt >= 2 branch of
// chapterCrossed is exercised here: a single addLore(2) must queue both
// crossed chapters, a crossing beyond the final chapter queues none, and a
// later event never requeues an already-passed chapter.
func TestSagaMultiCross(t *testing.T) {
	deck := []cardset.Entry{{Name: "History of Benalia", Count: 1}, {Name: "Plains", Count: 1}}
	set := cardsettest.Set(t, "saga")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.perm(id["History of Benalia"], 0)
	sl := sc.slot(o)
	p := &sc.s.perms[sl]
	if n := p.ctrN(sc.s, int(kop.CtrLore)); n != 1 {
		t.Fatalf("a Saga enters with %d lore counters, want 1", n)
	}
	// pendingChapters reports the chapter numbers of the queued triggers.
	pendingChapters := func() []int {
		t.Helper()
		var out []int
		for i := 0; i < int(sc.s.npend); i++ {
			pt := &sc.s.pend[i]
			if pt.card != id["History of Benalia"] {
				t.Fatalf("pending trigger from %s, want the Saga", tbl.Cards[pt.card].Name)
			}
			out = append(out, int(tbl.Cards[pt.card].Abilities[pt.ab].Gate.N))
		}
		return out
	}
	cross := func(from, amt int, want ...int) {
		t.Helper()
		if !p.setCtrN(sc.s, int(kop.CtrLore), from) {
			t.Fatalf("set the lore count to %d", from)
		}
		sc.s.npend = 0
		if !sc.s.addLore(sc.k, sl, amt) {
			t.Fatalf("addLore(%d) from %d placed nothing", amt, from)
		}
		if got := pendingChapters(); !slices.Equal(got, want) {
			t.Fatalf("addLore(%d) from lore %d queued chapters %v, want %v", amt, from, got, want)
		}
		if n := p.ctrN(sc.s, int(kop.CtrLore)); n != from+amt {
			t.Fatalf("lore count %d, want %d", n, from+amt)
		}
	}
	cross(1, 2, 2, 3) // entry count + a CounterNum$ 2: chapters II and III
	cross(0, 2, 1, 2) // from zero: chapters I and II
	cross(3, 2)       // past the final chapter: nothing is queued
}
