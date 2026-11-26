package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCrewAltCostLowers: Heart of Kiran's "You may remove a loyalty counter
// from a planeswalker you control rather than pay this Vehicle's crew cost"
// (S:Mode$ AlternativeCost | ValidSA$ Activated.Crew+Vehicle) lowers instead
// of refusing "AlternativeCost shape": the card keeps its crew ability and
// gains a second activated ability, its alternative (kop.Ability.AltOf names
// the crew ability), whose cost removes one loyalty counter from among the
// controller's planeswalkers and taps nothing.
func TestCrewAltCostLowers(t *testing.T) {
	all, bs := keclBuild(t, []string{"Heart of Kiran"})
	i := -1
	for j, c := range all {
		if c.Name == "Heart of Kiran" {
			i = j
		}
	}
	if i < 0 {
		t.Fatal("Heart of Kiran: not lowered at all")
	}
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Heart of Kiran refused: %v", r)
	}
	var crew, alt *kop.Ability
	for j := range bs[i].def.Abilities {
		a := &bs[i].def.Abilities[j]
		switch {
		case a.Kind == kop.AbActivated && a.AltOf == 0 && a.Cost.Flags&kop.CostCrew != 0:
			crew = a
		case a.Kind == kop.AbActivated && a.AltOf != 0:
			alt = a
		}
	}
	if crew == nil || alt == nil {
		t.Fatalf("crew ability %v, alternative %v: want both", crew != nil, alt != nil)
	}
	if got := &bs[i].def.Abilities[alt.AltOf-1]; got != crew {
		t.Errorf("AltOf %d names another ability than the crew ability", alt.AltOf)
	}
	c := alt.Cost
	if c.Flags != kop.CostSubAmong || c.N != 1 || c.Ctr != kop.CtrLoyalty || c.Filter == 0 || c.Mana != (kop.ManaCost{}) {
		t.Errorf("alternative cost = flags %b N %d ctr %d filter %d mana %+v, want CostSubAmong/1/loyalty/a filter/no mana",
			c.Flags, c.N, c.Ctr, c.Filter, c.Mana)
	}
	if crew.Cost.Flags&kop.CostTapPower == 0 || len(alt.Prog) != len(crew.Prog) {
		t.Errorf("crew cost flags %b, alternative body %d instrs vs %d: want the tap-power cost and the same body",
			crew.Cost.Flags, len(alt.Prog), len(crew.Prog))
	}
}
