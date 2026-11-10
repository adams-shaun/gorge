package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A fallback window's filter answers its colour after its fee, and the fee
// spends the floating restricted mana the activation admits before any
// ordinary unit (filtDo, gorge's carve): Abzan Devotee's {1} takes the
// Optimizer's restricted {U} and keeps the {W}, so for {1}{B} the {W} covers
// the generic, the clamp is decided and the colour is {B} (CR 601.2g, 106.6).
// A simulation that spent the {W} instead leaves the window undecided and
// answers the lowest colour {W}, stranding the {B}.
func TestFallbackFilterFeeSpendsRestrictedManaFirst(t *testing.T) {
	deck := []cardset.Entry{{Name: "Abzan Devotee", Count: 1}, {Name: "Guidelight Optimizer", Count: 1},
		{Name: "Doom Blade", Count: 1}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	set := cardsettest.Set(t, "filtfallbackrestr")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	opt := sc.slot(sc.perm(id["Guidelight Optimizer"], 0))
	dev := sc.slot(sc.perm(id["Abzan Devotee"], 0))
	var held [kop.NumManaKinds]uint8
	sc.s.tapGap(sc.k, 0, opt, slotBit(opt), &held)
	sc.s.pl[0].pool[kop.W] = 1
	if sc.s.restrN(0) != 1 {
		t.Fatalf("restricted %v, want the Optimizer's one floating {U}", sc.s.rb[0])
	}
	if !sc.s.windowFiltFirstUsable(sc.k, 0, dev) {
		t.Fatal("the Devotee's filter is not usable over the floating {W}")
	}
	cost := kop.ManaCost{Generic: 1}
	cost.Pips[kop.B] = 1
	sc.s.windowFiltFirstFallback(sc.k, 0, dev, &cost)
	if sc.s.rbAny(0) {
		t.Errorf("restricted %v, want the restricted {U} spent on the fee", sc.s.rb[0])
	}
	if got := sc.s.pl[0].pool; got[kop.W] != 1 || got[kop.B] != 1 {
		t.Fatalf("pool %v, want the {W} kept and the colour {B} (the owed pip)", got)
	}
}
