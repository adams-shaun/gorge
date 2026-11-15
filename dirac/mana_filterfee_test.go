package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFiltFeeUnitSlotOrder: Darkwater Catacombs ({1},{T}: add {U}{B}) beside a
// Swamp and a Forest pays {X}{B}{B}'s window the same whichever land sits first
// in slot order. The fee is paid by the Forest and the Catacombs' B joins the
// Swamp's, so X reaches 1 (gorge's castWindowPays); scoring each fee unit before
// the filter's own output is added reads every unit as -1 and takes the first
// group, the Swamp's B, which left no X at all when the Swamp came first.
func TestFiltFeeUnitSlotOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Darkwater Catacombs", Count: 1}, {Name: "Swamp", Count: 8}, {Name: "Forest", Count: 8}}
	set := cardsettest.Synth(t, "tail-08")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var cost kop.ManaCost
	cost.X = 1
	cost.Pips[2] = 2 // {X}{B}{B}
	for _, order := range [][]string{{"Swamp", "Forest"}, {"Forest", "Swamp"}} {
		sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
		for _, n := range order {
			sc.perm(id[n], 0)
		}
		cat := sc.slot(sc.perm(id["Darkwater Catacombs"], 0))
		s, k := sc.s, sc.k
		if !s.scriptFilt(k, 0).has(cat) {
			t.Fatalf("order %v: Catacombs is no filter gap", order)
		}
		var av avail
		s.availFor(k, 0, &av)
		if mx := av.maxX(&cost, pipMaskOf(&cost)); mx != -1 {
			t.Fatalf("order %v: plain maxX %d, want -1 (BB is not payable without the filter)", order, mx)
		}
		if got := s.filtMaxX(k, 0, &cost, &av, false); got != 1 {
			t.Errorf("order %v: filtMaxX %d, want 1", order, got)
		}
	}
}
