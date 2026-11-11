package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSoleLifeTapMana: a card's only mana ability "{T}, Pay N life: Add one
// mana of any color" (Mana Confluence) lowers whole, with the life as its
// cost; a second mana ability beside it, a restriction or a fee still refuses.
func TestSoleLifeTapMana(t *testing.T) {
	const head = "Name:Life Probe\nManaCost:no cost\nTypes:Land\n"
	def, rs := kwardpowBuild(t, "Life Probe", head+"A:AB$ Mana | Cost$ T PayLife<1> | Produced$ Any | Amount$ 1 | SpellDescription$ x.\n")
	if len(rs) != 0 {
		t.Fatalf("Mana Confluence's shape refused: %v", rs)
	}
	found := false
	for _, a := range def.Abilities {
		if a.Kind == kop.AbMana {
			found = true
			if a.Cost.Flags != kop.CostTap|kop.CostPayLife || a.Cost.N != 1 {
				t.Errorf("mana ability cost flags %v N %d, want {T} + PayLife 1", a.Cost.Flags, a.Cost.N)
			}
		}
	}
	if !found {
		t.Fatal("no mana ability lowered")
	}
	for _, c := range []struct{ name, src string }{
		{"a first life ability beside a second", head + "A:AB$ Mana | Cost$ T PayLife<1> | Produced$ G | Amount$ 1 | SpellDescription$ x.\nA:AB$ Mana | Cost$ T | Produced$ C | Amount$ 1 | SpellDescription$ y.\n"},
		{"a fee beside the life", head + "A:AB$ Mana | Cost$ 1 T PayLife<1> | Produced$ Any | Amount$ 1 | SpellDescription$ x.\n"},
	} {
		_, rs := kwardpowBuild(t, "Life Probe", c.src)
		if len(rs) == 0 {
			t.Errorf("%s: lowered, want a refusal", c.name)
			continue
		}
	}
	for _, c := range lowerNamed(t, []string{"Mana Confluence"}) {
		if len(c.Refusals) > 0 {
			t.Errorf("%s: refused: %v", c.Name, c.Refusals)
		}
	}
}
