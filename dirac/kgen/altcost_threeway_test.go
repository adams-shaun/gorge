package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltCostThreeWayLowers: Dusk Mangler's three-way
// AlternateAdditionalCost "Sac<1/Creature>:Discard<1/Card>:PayLife<4>"
// lowers as the either-or's three parts — the spell's first part
// (CardDef.SpellCost, a creature to sacrifice), the second
// (CardDef.AltAddCost, a card to discard) and the third
// (CardDef.AltAddCost2, four life). The kernel's ask (cast.go
// altAddOptions) offers all three, the payable ones first in script
// order, as gorge's altAddAsk orders its parts (CR 601.2h).
func TestAltCostThreeWayLowers(t *testing.T) {
	all, bs := keclBuild(t, []string{"Dusk Mangler"})
	i := -1
	for j, c := range all {
		if c.Name == "Dusk Mangler" {
			i = j
		}
	}
	if i < 0 {
		t.Fatal("Dusk Mangler: not lowered at all")
	}
	if r := lowerRefusals(all[i]); len(r) != 0 {
		t.Fatalf("Dusk Mangler refused: %v", r)
	}
	d := bs[i].def
	if !d.HasAltAdd {
		t.Fatal("Dusk Mangler: HasAltAdd unset")
	}
	sc := d.SpellCost
	if sc.Flags != kop.CostSac || sc.N != 1 || sc.Mana != (kop.ManaCost{}) {
		t.Errorf("SpellCost (part one) = flags %b N %d mana %+v, want CostSac/1/zero", sc.Flags, sc.N, sc.Mana)
	}
	ac := d.AltAddCost
	if ac.Flags != kop.CostDiscard || ac.N != 1 || ac.Mana != (kop.ManaCost{}) {
		t.Errorf("AltAddCost (part two) = flags %b N %d mana %+v, want CostDiscard/1/zero", ac.Flags, ac.N, ac.Mana)
	}
	ac2 := d.AltAddCost2
	if ac2.Flags != kop.CostPayLife || ac2.N != 4 || ac2.Mana != (kop.ManaCost{}) {
		t.Errorf("AltAddCost2 (part three) = flags %b N %d mana %+v, want CostPayLife/4/zero", ac2.Flags, ac2.N, ac2.Mana)
	}
}
