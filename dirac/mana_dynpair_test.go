package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDynPairUnit: Itlimoc, Cradle of the Sun ({T}: Add {G}, then {T}: Add {G}
// for each creature you control) is one planner source with two alternatives
// when the count is above one: {G} and {G} x the count (gorge's evaluated-amount
// layer joins the second to the source's unit, and the planner takes the best
// per payment). A count of 0 or 1 is no larger alternative: the plain view.
func TestDynPairUnit(t *testing.T) {
	set := cardsettest.Set(t, "dynpair")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		id[tbl.Cards[c].Name] = CardID(c)
	}
	const back = "face:Growing Rites of Itlimoc#1"
	for _, n := range []string{back, "Grizzly Bears", "Island", "Forest"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("%s not in the dynpair table", n)
		}
	}
	for _, creatures := range []int{0, 1, 2, 5} {
		sc := newScenTableLib(t, tbl, id["Forest"], Config{})
		if !sc.k.gen.anyGatePair {
			t.Fatal("anyGatePair unset: Itlimoc is not a pair in the dynpair prep")
		}
		it := sc.perm(id[back], 0)
		sc.perm(id["Island"], 0)
		for i := 0; i < creatures; i++ {
			sc.perm(id["Grizzly Bears"], 0)
		}
		v := sc.s.manaPCGen(sc.k, sc.slot(it))
		wantWide := uint8(0)
		if creatures > 1 {
			wantWide = uint8(creatures) // 2 and 5 differ from the plain {G} (1): the larger alternative exists
		}
		if v.wideN != wantWide || v.manaN != 1 {
			t.Errorf("%d creatures: view manaN %d wideN %d, want 1 and %d", creatures, v.manaN, v.wideN, wantWide)
			continue
		}
		var pk planner
		pk.dry = true
		c := cost(0, kop.G)
		sc.s.tapForPK(sc.k, 0, &c, 0, NoObj, false, false, &pk)
		if !pk.found {
			t.Fatalf("%d creatures: no plan for {G}", creatures)
		}
		var ns []uint8
		for ti := 0; ti < pk.nt; ti++ {
			if pk.types[ti].pair != 0 {
				ns = append(ns, pk.types[ti].n)
			}
		}
		switch {
		case wantWide == 0 && (pk.pairs || len(ns) != 0):
			t.Errorf("%d creatures: planner pair %v types %v, want the plain unit", creatures, pk.pairs, ns)
		case wantWide != 0 && (!pk.pairs || len(ns) != 2 || ns[0] != 1 || ns[1] != wantWide):
			t.Errorf("%d creatures: planner pair %v alternatives %v, want {G} x1 and {G} x%d", creatures, pk.pairs, ns, wantWide)
		}
	}
}
