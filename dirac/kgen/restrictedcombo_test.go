package kgen

import "testing"

// comboAb is a plain {T} mana ability of colours cols and literal amount n,
// a Combo choice when combo, spell-restricted when restr.
func comboAb(cols string, n int, combo, restr bool) *lAbility {
	a := &lAbility{Mana: &lMana{}}
	a.Cost.Tap = true
	a.Mana.Colors, a.Mana.Choice = cols, combo
	a.Mana.Amount = lValue{Kind: vConst, N: n}
	if restr {
		a.Mana.Restrict = &lFilter{}
	}
	return a
}

// TestRestrictedComboShape: a restricted {T} Combo of any colours lowers
// beside a plain {T} one mana of any colour only at amount two with a
// spell-only restriction (Hermitic Herbalist's shape); any other amount,
// colour set, restriction or first ability stays refused.
func TestRestrictedComboShape(t *testing.T) {
	first := comboAb("WUBRG", 1, false, false)
	if !plainTapMana(first, false) || !unrestrictedOne(first) {
		t.Fatal("the first ability is no plain unrestricted {T} one: the case reads nothing")
	}
	if !manaGapModelled(first, comboAb("WUBRG", 2, true, true)) {
		t.Error("the restricted Combo of any colours, amount 2, beside {T}: any colour was refused")
	}
	abAlso := comboAb("WUBRG", 2, true, true)
	abAlso.Mana.RestrictAb = &lFilter{}
	nonSpell := comboAb("WUBRG", 2, true, true)
	nonSpell.Mana.NonSpell = true
	for name, c := range map[string]struct{ first, a *lAbility }{
		"amount 3":            {first, comboAb("WUBRG", 3, true, true)},
		"amount 4":            {first, comboAb("WUBRG", 4, true, true)},
		"two colours":         {first, comboAb("WU", 2, true, true)},
		"unrestricted":        {first, comboAb("WUBRG", 2, true, false)},
		"activation restrict": {first, abAlso},
		"non-spell restrict":  {first, nonSpell},
		"fixed first colour":  {comboAb("G", 1, false, false), comboAb("WUBRG", 2, true, true)},
		"combo first":         {comboAb("WUBRG", 1, true, false), comboAb("WUBRG", 2, true, true)},
	} {
		if manaGapModelled(c.first, c.a) {
			t.Errorf("%s: accepted", name)
		}
	}
}

// TestHermiticHerbalistLowers: the card itself lowers (corpus; skips without it).
func TestHermiticHerbalistLowers(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Hermitic Herbalist"})
	n := 0
	for _, c := range lw.cards {
		if c.Name == "Hermitic Herbalist" {
			n++
			if len(c.Refusals) != 0 {
				t.Errorf("Hermitic Herbalist refused: %v", c.Refusals)
			}
		}
	}
	if n != 1 {
		t.Fatalf("Hermitic Herbalist lowered %d times", n)
	}
}
