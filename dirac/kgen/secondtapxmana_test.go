package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSecondTapXManaAbilityLowers: a plain "{T}: Add {C}" beside "{T}, tap an
// untapped creature you control: Add one mana of any color" (Scene of the
// Crime, Holdout Settlement, Survivors' Encampment) lowers with no refusal,
// and the built definition's second mana ability is the tap-another shape the
// kernel reads as the card's gap (kop.SecondTapXManaAb; kernel
// mana_tapanother.go tapXAbOf), so kop.ManaAbilityHonoured admits it.
func TestSecondTapXManaAbilityLowers(t *testing.T) {
	want := []string{"Scene of the Crime", "Holdout Settlement", "Survivors' Encampment"}
	lw := lowerNamedPool(t, want)
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for i, c := range all {
		seen[c.Name] = true
		if rs := lowerRefusals(c); len(rs) > 0 {
			t.Errorf("%s refused: %v", c.Name, rs)
			continue
		}
		cd := &bs[i].def
		first, second := -1, -1
		for j := range cd.Abilities {
			if cd.Abilities[j].Kind != kop.AbMana {
				continue
			}
			if first < 0 {
				first = j
			} else if second < 0 {
				second = j
			}
			if !kop.ManaAbilityHonoured(cd, j) {
				t.Errorf("%s: mana ability %d (%s) not honoured", c.Name, j, kop.ManaCostShape(&cd.Abilities[j]))
			}
		}
		if first < 0 || second < 0 {
			t.Fatalf("%s: mana abilities %d and %d, want two", c.Name, first, second)
		}
		if f := &cd.Abilities[first]; f.Cost.Flags != kop.CostTap || f.ManaColors != kop.ColC {
			t.Errorf("%s: first mana ability %s producing %v, want a plain {T}: Add {C}", c.Name, kop.ManaCostShape(f), f.ManaColors)
		}
		if !kop.SecondTapXManaAb(cd, first, second) {
			t.Errorf("%s: second mana ability %s is not the tap-another shape beside a plain first", c.Name, kop.ManaCostShape(&cd.Abilities[second]))
		}
	}
	for _, n := range want {
		if !seen[n] {
			t.Errorf("%s not lowered", n)
		}
	}
}
