package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerLifeAndFeeManaAbilities: the {T}-plus-life and {T}-plus-mana mana
// abilities of the imported Commander decks lower. The Horizon lands' "{T}, Pay
// 1 life: Add {G} or {W}" is a sole paid-life ability with a colour choice
// (kop.SoleLifeManaAb; gorge's last-resort Consequence{Life: 1} per colour), and
// Cabal Coffers' and Magus of the Coffers' "{2}, {T}: Add {B} for each Swamp you
// control" a fee-funded evaluated amount (kop.FeeDynManaAb, a first-ability
// filter). Selvala's "{G}, {T}: Add X mana in any combination of colors" stays
// refused with its measured reason: the kernel's combination search
// (mana_comboany.go) has no fee-funded form, and a Combo Any source beside a
// filter stalls the table (StallComboAnyMix).
func TestLowerLifeAndFeeManaAbilities(t *testing.T) {
	lowered := []string{"Cabal Coffers", "Magus of the Coffers", "Horizon Canopy", "Fiery Islet", "Waterlogged Grove",
		"Nurturing Peatland", "Sunbaked Canyon", "Silent Clearing"}
	refused := map[string]string{
		"Selvala, Heart of the Wilds": "a fee-funded Combo Any production",
	}
	var names []string
	names = append(names, lowered...)
	for n := range refused {
		names = append(names, n)
	}
	lw := lowerNamedPool(t, names)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for i, c := range all {
		if c.Token {
			continue
		}
		seen++
		if why, ok := refused[c.Name]; ok {
			got := lowerRefusals(c)
			if len(got) != 1 || !strings.Contains(got[0].Reason, why) {
				t.Errorf("%s: want the single refusal %q, got %v", c.Name, why, got)
			}
			continue
		}
		if got := lowerRefusals(c); len(got) > 0 {
			t.Errorf("%s refused: %v", c.Name, got)
			continue
		}
		honoured := 0
		cd := &bs[i].def
		for j := range cd.Abilities {
			if cd.Abilities[j].Kind != kop.AbMana {
				continue
			}
			honoured++
			if !kop.ManaAbilityHonoured(cd, j) {
				t.Errorf("%s: mana ability %d (%s) is not honoured", c.Name, j, kop.ManaCostShape(&cd.Abilities[j]))
			}
		}
		if honoured != 1 {
			t.Errorf("%s: %d mana abilities lowered, want 1", c.Name, honoured)
		}
	}
	if seen != len(names) {
		t.Errorf("built %d of %d cards", seen, len(names))
	}
}
