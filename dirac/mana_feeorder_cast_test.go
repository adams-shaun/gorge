package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestFeeOrderFallbackOffersFilterPaidCast: The Astonishing Ant-Man's {G}{U}
// over Forest, Forest, Training Compound, Asgardian Citadel and Baxter
// Building. The blue comes only from Baxter Building's "{4}, {T}: Add four
// mana in any combination of colors", whose fee takes every other land.
// gorge's plain prefix search spends its 160 nodes on the lands' colour asks
// before the allocation that pays (dirac's needs 165), and its phased run in
// the fee order (funding, then the multi-unit fee gap, then the free gaps:
// scriptSearch.feeOrder) finds the line in 17 nodes, so gorge offers the cast.
func TestFeeOrderFallbackOffersFilterPaidCast(t *testing.T) {
	set := cardsettest.Set(t, "feeordercast")
	deck := []cardset.Entry{{Name: "The Astonishing Ant-Man", Count: 1}, {Name: "Baxter Building", Count: 1},
		{Name: "Training Compound", Count: 1}, {Name: "Asgardian Citadel", Count: 1}, {Name: "Forest", Count: 8}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	for _, tc := range []struct {
		name  string
		lands []string
		want  bool
	}{
		{"compound and citadel", []string{"Forest", "Forest", "Training Compound", "Asgardian Citadel", "Baxter Building"}, true},
		{"two compounds", []string{"Forest", "Forest", "Training Compound", "Training Compound", "Baxter Building"}, true},
		{"plain lands", []string{"Forest", "Forest", "Training Compound", "Forest", "Baxter Building"}, true},
		{"fee unfunded", []string{"Forest", "Forest", "Training Compound", "Baxter Building"}, false},
	} {
		sc := newScenTableLib(t, tbl, da[0], Config{})
		for _, n := range tc.lands {
			o := sc.perm(id[n], 0)
			sc.s.perms[sc.slot(o)].flags &^= pfTapped // Asgardian Citadel enters tapped
		}
		am := sc.hand(id["The Astonishing Ant-Man"], 0)
		d := sc.s.Step()
		got := false
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkCast && Obj(a.Source()) == am {
				got = true
			}
		}
		if got != tc.want {
			t.Errorf("%s: Ant-Man cast offered %v, want %v", tc.name, got, tc.want)
		}
	}
}
