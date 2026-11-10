package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The mana shapes of the artifact-rider family stay refused by name, each for
// the reason the measurement found. A lockstep sweep with the shape lowered
// diverged (or the kernel has no mechanism), so a card here is lowered only
// when the mechanism and a lockstep repro land together. A filter whose cost
// sacrifices its source (Omni-Cheese Pizza) is one that landed: the kernel
// sacrifices the source as the filter activation is applied.
var manaRiderForms = []formCase{
	{card: "Boommobile", form: "an artifact's triggered mana restricted to activated abilities", lir: "ManaChoice A=4 C=1 CS=WUBRG F={types=0x0 special=0x20}"},
}

func TestManaRiderShapes(t *testing.T) {
	checkForms(t, manaRiderForms)
	// Omni-Cheese Pizza lowers: its first mana ability is a kernel filter
	lw := lowerNamedPool(t, []string{"Omni-Cheese Pizza"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range all {
		if c.Name != "Omni-Cheese Pizza" {
			continue
		}
		if r := lowerRefusals(c); len(r) != 0 {
			t.Errorf("Omni-Cheese Pizza: want no refusal, got %v", r)
		}
		first := -1
		for j := range bs[i].def.Abilities {
			if bs[i].def.Abilities[j].Kind == kop.AbMana {
				first = j
				break
			}
		}
		if first < 0 || !kop.FilterManaAb(&bs[i].def.Abilities[first]) {
			t.Errorf("Omni-Cheese Pizza: first mana ability %d is not kop.FilterManaAb", first)
		}
		return
	}
	t.Error("Omni-Cheese Pizza: not built")
}
