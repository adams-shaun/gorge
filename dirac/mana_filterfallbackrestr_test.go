package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A fallback window's filter answers its colour after its fee, and the fee is
// paid as gorge's payment spends it (filtDo, filtFeeSpend): the visible pool in
// C W U B R G order, the restricted mana the activation admits in line with the
// plain units, so the plain {W} comes before the Optimizer's restricted {U}
// (ResolveMana's plain-first unit search; CR 601.2g, 106.6). Abzan Devotee's
// {1} for {1}{B} therefore takes the {W} and keeps the {U}; the restricted {U}
// cannot pay Doom Blade, so the clamp is undecided and the colour is the
// lowest, {W}.
func TestFallbackFilterFeeSpendsPlainManaInColourOrder(t *testing.T) {
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
	if sc.s.restrN(0) != 1 {
		t.Errorf("restricted %v, want the restricted {U} kept: the fee takes the plain {W} first", sc.s.rb[0])
	}
	if got := sc.s.pl[0].pool; got[kop.W] != 1 || got[kop.B] != 0 {
		t.Fatalf("pool %v, want the filter's {W} (the fee spent the {W}; the clamp is undecided, so the lowest colour)", got)
	}
}
