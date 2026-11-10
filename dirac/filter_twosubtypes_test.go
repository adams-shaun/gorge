package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTwoSubtypeFilter: a clause naming two subtypes is Sub plus
// PredSubtype (Radioactive Spider's "Spider.Hero" search): a card holding
// both matches, one holding only the first does not, in a library and on the
// battlefield, and the matchup's compact subtype table renumbers the second
// subtype with Sub (PredN is an index, not a CardID).
func TestTwoSubtypeFilter(t *testing.T) {
	deck := []cardset.Entry{{Name: "Radioactive Spider", Count: 1}, {Name: "Lady Spider, Maybelle Reilly", Count: 1},
		{Name: "Iron Spider, Stark Upgrade", Count: 1}, {Name: "Forest", Count: 8}}
	set := cardsettest.Set(t, "twosubtype")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	// the filter is the one kgen lowered onto Radioactive Spider's search, read
	// out of the matchup's compact table, so it has been through the remap
	var f kop.FilterID
	for _, ab := range tbl.Cards[id["Radioactive Spider"]].Abilities {
		for _, in := range ab.Prog {
			if in.Op == kop.OpSearchEx {
				f = in.F
			}
		}
	}
	if f == 0 {
		t.Fatal("Radioactive Spider has no lowered search")
	}
	if fl := tbl.Filters[f]; fl.Sub == 0 || fl.Pred != kop.PredSubtype {
		t.Fatalf("the search filter is not Sub + PredSubtype: %+v", fl)
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Forest"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	s.turn, s.active, s.step, s.ctl = 2, 0, kop.StepMain1, ctlPriority
	sc := &scen{s: s, k: s.k()}
	for _, c := range []struct {
		name string
		want bool
	}{{"Lady Spider, Maybelle Reilly", true}, {"Iron Spider, Stark Upgrade", true}, {"Radioactive Spider", false}, {"Forest", false}} {
		if got := sc.s.matchCard(sc.k, f, id[c.name], 0, 0); got != c.want {
			t.Errorf("library %s: got %v want %v", c.name, got, c.want)
		}
		if got := sc.s.matchPerm(sc.k, f, sc.slot(sc.perm(id[c.name], 0)), 0, NoSlot); got != c.want {
			t.Errorf("battlefield %s: got %v want %v", c.name, got, c.want)
		}
	}
}
