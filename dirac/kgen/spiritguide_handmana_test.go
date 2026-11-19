package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSpiritGuideHandMana: a Spirit Guide's hand-zone mana ability (exile
// this card from your hand: add one fixed colour) lowers onto
// CardDef.HandMana, its colour, with no ability and no refusal; the other ExileFromHand<1/CARDNAME> shapes (a non-mana hand
// ability) keep their own refusal and no HandMana.
func TestSpiritGuideHandMana(t *testing.T) {
	want := map[string]uint8{"Simian Spirit Guide": uint8(kop.ColR), "Elvish Spirit Guide": uint8(kop.ColG)}
	cs := lowerNamed(t, []string{"Simian Spirit Guide", "Elvish Spirit Guide", "Masked Bandits"})
	seen := map[string]bool{}
	for _, c := range cs {
		seen[c.Name] = true
		rs := lowerRefusals(c)
		if col, ok := want[c.Name]; ok {
			if c.HandMana != col || len(c.Abilities) != 0 || len(rs) != 0 {
				t.Errorf("%s: HandMana %#x (want %#x), %d abilities, refusals %v", c.Name, c.HandMana, col, len(c.Abilities), rs)
			}
			continue
		}
		if c.HandMana != 0 {
			t.Errorf("%s: a non-mana hand ability took HandMana", c.Name)
		}
		refused := false
		for _, r := range rs {
			refused = refused || strings.Contains(r.Reason, "exile cost")
		}
		if !refused {
			t.Errorf("%s: its exile-cost hand ability lost its refusal: %v", c.Name, rs)
		}
	}
	for _, n := range []string{"Simian Spirit Guide", "Elvish Spirit Guide", "Masked Bandits"} {
		if !seen[n] {
			t.Errorf("%s was not lowered at all", n)
		}
	}
}
