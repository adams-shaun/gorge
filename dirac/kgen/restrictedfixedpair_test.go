package kgen

import (
	"strings"
	"testing"
)

// TestRestrictedFixedColourPairShape: a spell-restricted {T} ability of two to
// four mana of the first ability's own fixed colour is the kernel's restricted
// pair unit (Tablet of Discovery's {R}{R} beside {R}); a different colour, an
// amount above four, a Combo, an unrestricted or activation-restricted second
// ability and a Combo first ability stay refused, the colour case with the
// several-mana refusal text.
func TestRestrictedFixedColourPairShape(t *testing.T) {
	first := comboAb("R", 1, false, false)
	if !plainTapMana(first, false) || !unrestrictedOne(first) {
		t.Fatal("the first ability is no plain unrestricted {T} one: the case reads nothing")
	}
	for _, n := range []int{2, 3, 4} {
		if !manaGapModelled(first, comboAb("R", n, false, true)) {
			t.Errorf("restricted {R} x%d beside {R}: refused", n)
		}
	}
	abAlso := comboAb("R", 2, false, true)
	abAlso.Mana.RestrictAb = &lFilter{}
	nonSpell := comboAb("R", 2, false, true)
	nonSpell.Mana.NonSpell = true
	firstCombo := comboAb("R", 1, true, false)
	cases := map[string]struct{ first, a *lAbility }{
		"amount 5":            {first, comboAb("R", 5, false, true)},
		"other colour":        {comboAb("G", 1, false, false), comboAb("R", 2, false, true)},
		"combo":               {first, comboAb("R", 2, true, true)},
		"unrestricted":        {first, comboAb("R", 2, false, false)},
		"activation restrict": {first, abAlso},
		"non-spell restrict":  {first, nonSpell},
		"combo first":         {firstCombo, comboAb("R", 2, false, true)},
	}
	for name, c := range cases {
		if manaGapModelled(c.first, c.a) {
			t.Errorf("%s: accepted", name)
		}
	}
	if why := manaGapWhy(cases["other colour"].first, cases["other colour"].a); !strings.Contains(why, "a restricted fixed colour of several mana") {
		t.Errorf("the other-colour refusal %q does not name the restricted fixed colour of several mana", why)
	}
}

// TestTabletOfDiscoveryLowers: the card itself lowers (corpus; skips without it).
func TestTabletOfDiscoveryLowers(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Tablet of Discovery"})
	n := 0
	for _, c := range lw.cards {
		if c.Name == "Tablet of Discovery" {
			n++
			for _, r := range c.Refusals {
				if strings.Contains(r.Reason, "mana ability") {
					t.Errorf("Tablet of Discovery refused on its mana: %v", r)
				}
			}
			if len(c.Refusals) != 0 {
				t.Errorf("Tablet of Discovery refused: %v", c.Refusals)
			}
		}
	}
	if n != 1 {
		t.Fatalf("Tablet of Discovery entries = %d, want 1", n)
	}
}
