package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFiltFreeTapPainFailsOpen: the free-source scan behind the fee-only
// colour-ask clamp (filtFeeColStalls's second cut) must refuse a source whose
// bare-tap ability also pays life (Mana Confluence's "{T}, Pay 1 life"): the
// swapped line would spend that life, which the dominated branch saved, so
// the endpoint after both activations is not the same and the cut fails open
// (canTapForManaPC only holds the lethal band out, life < painN). gorge's
// scan refuses the same class on the cost's parsed life
// (pay.ManaFreeCost's c.Life == 0; engine/rules'
// potential_feeonly_unusable_free_test.go pins it beside this mirror).
// Llanowar Elves' plain "{T}: Add {G}" is the contrast the scan must still see.
func TestFiltFreeTapPainFailsOpen(t *testing.T) {
	deck := []cardset.Entry{{Name: "Mana Confluence", Count: 2}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Serra Angel", Count: 1}, {Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8}}
	set := cardsettest.Set(t, "manaconfluence")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	conf := sc.perm(id["Mana Confluence"], 0)
	elves := sc.perm(id["Llanowar Elves"], 0)
	s, k := sc.s, sc.k
	if int(s.pl[SeatIx(0)].life) < 1 {
		t.Fatal("precondition: the payer is at 0 life")
	}
	pcConf := &k.cards[s.cardAt(sc.slot(conf))]
	if pcConf.manaAb < 0 || !pcConf.manaTap || !pcConf.painLife {
		t.Fatalf("precondition: Mana Confluence is no tap-plus-life census source (manaAb %d manaTap %v painLife %v)",
			pcConf.manaAb, pcConf.manaTap, pcConf.painLife)
	}
	if s.srcMaskAt(pcConf, sc.slot(conf))&(1<<kop.W) == 0 {
		t.Fatal("precondition: Mana Confluence cannot make W")
	}
	if s.filtFreeTapColour(k, 0, NoSlot, int(kop.W)) {
		t.Fatal("the scan counted Mana Confluence's paid-life ability as a free bare tap")
	}
	pcElves := &k.cards[s.cardAt(sc.slot(elves))]
	if pcElves.manaAb < 0 || !pcElves.manaTap || pcElves.painLife {
		t.Fatalf("precondition: Llanowar Elves are no plain tap census source (manaAb %d manaTap %v painLife %v)",
			pcElves.manaAb, pcElves.manaTap, pcElves.painLife)
	}
	if !s.filtFreeTapColour(k, 0, NoSlot, int(kop.G)) {
		t.Fatal("precondition: the scan must still see Llanowar Elves' plain tap")
	}
}
