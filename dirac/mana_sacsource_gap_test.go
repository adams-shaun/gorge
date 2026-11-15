package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A sacrifice-only mana ability (an Eldrazi Scion's "Sacrifice this
// creature: Add {C}") is a last-resort unit of gorge's census, as a
// Treasure's {T}-and-sacrifice one is (pay.PlanLastResortCostOK): it is no
// census gap, so a filter beside it keeps the board filter-only.
func TestCensusGapManaSacOnlyIsNoGap(t *testing.T) {
	t.Parallel()
	ab := func(f kop.CostFlags) *kop.Ability {
		a := &kop.Ability{Kind: kop.AbMana, ManaColors: kop.ColC, ManaN: 1}
		a.Cost.Flags = f
		return a
	}
	if !censusGapMana(ab(kop.CostTap | kop.CostPayLife)) {
		t.Fatal("precondition: a {T}, pay-life ability is a census gap")
	}
	if censusGapMana(ab(kop.CostTap | kop.CostSacSelf)) {
		t.Fatal("precondition: a Treasure's {T}, sacrifice ability is no census gap")
	}
	if censusGapMana(ab(kop.CostSacSelf)) {
		t.Fatal("a sacrifice-only mana ability must be no census gap (gorge prices it last resort)")
	}
}
