package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWindowOwedCostCountsAdmittedRestrictedMana: a window source's colour ask
// prices the pool AvailableFor holds, which includes the floating restricted
// mana the payment may spend (CR 106.6). A {U} pip covered by Hydraulic
// Helper's restricted {U} is not owed, so the source answers another pip.
// Without windowOwedCost the ask still sees the pip and the window strands a
// hybrid cast (Head of the Homestead beside Helga, Skittish Seer).
func TestWindowOwedCostCountsAdmittedRestrictedMana(t *testing.T) {
	deck := []cardset.Entry{{Name: "Hydraulic Helper", Count: 2}, {Name: "Island", Count: 16}}
	set := cardsettest.Synth(t, "std-msh")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	helper := id["Hydraulic Helper"]
	sl := sc.slot(sc.perm(helper, 0))
	var held [kop.NumManaKinds]uint8
	sc.s.tapGap(sc.k, 0, sl, slotBit(sl), &held)
	if sc.s.restrN(0) == 0 {
		t.Fatal("precondition: no restricted mana floating")
	}
	o := sc.hand(helper, 0)
	sc.s.prio = 0
	sc.s.beginCast(sc.k, o, 0)
	if sc.s.restrPoolAdmits(sc.k, 0) == 0 {
		t.Fatal("precondition: the floating {U} does not admit the artifact spell")
	}
	cost := kop.ManaCost{Generic: 1}
	cost.Pips[kop.U] = 1
	if got := sc.s.windowOwedCost(sc.k, 0, &cost); got.Pips[kop.U] != 0 {
		t.Fatalf("owed {U} pips %d, want 0: the restricted {U} covers it", got.Pips[kop.U])
	}
}
