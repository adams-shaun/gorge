package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEachColourManaLowers: a mana ability naming several colours in its
// production ("W U": both at once, CR 106.1) lowers to ManaEach over their
// mask, the amount the count of each; every shape the kernel does not model
// (a cost beyond {T}, a spend restriction, a second mana ability, unequal
// counts, a loyalty ability) is refused by name.
func TestEachColourManaLowers(t *testing.T) {
	const head = "Name:Prism Probe\nManaCost:4\nTypes:Artifact\n"
	for _, c := range []struct {
		name, ab string
		col      kop.ColorMask
		n        uint8
	}{
		{"pair", "A:AB$ Mana | Cost$ T | Produced$ U W | SpellDescription$ Two colours.", kop.ColW | kop.ColU, 1},
		{"five", "A:AB$ Mana | Cost$ T | Produced$ G R B U W | SpellDescription$ Five colours.", kop.ColAny, 1},
		{"doubled", "A:AB$ Mana | Cost$ T | Produced$ R G R G | SpellDescription$ Two of each.", kop.ColR | kop.ColG, 2},
		{"spell restricted", "A:AB$ Mana | Cost$ T | Produced$ G U | RestrictValid$ Spell.Creature | SpellDescription$ Creatures only.", kop.ColG | kop.ColU, 1},
		{"colourless", "A:AB$ Mana | Cost$ T | Produced$ U C | SpellDescription$ Blue and colourless.", kop.ColU | kop.ColC, 1},
	} {
		def, rs := kwardpowBuild(t, "Prism Probe", head+c.ab+"\n")
		if len(rs) != 0 {
			t.Errorf("%s: refused %v", c.name, rs)
			continue
		}
		var got *kop.Ability
		for i := range def.Abilities {
			if def.Abilities[i].Kind == kop.AbMana {
				got = &def.Abilities[i]
			}
		}
		if got == nil || !got.ManaEach || got.ManaColors != c.col || got.ManaN != c.n || got.AFlags&kop.AfManaChoice != 0 {
			t.Errorf("%s: ability %+v, want ManaEach over %#x, %d each", c.name, got, c.col, c.n)
		}
	}
	// one colour named twice stays two mana of it, not ManaEach
	def, rs := kwardpowBuild(t, "Prism Probe", head+"A:AB$ Mana | Cost$ T | Produced$ B B | SpellDescription$ Two black.\n")
	if len(rs) != 0 || def.Abilities[0].ManaEach || def.Abilities[0].ManaColors != kop.ColB || def.Abilities[0].ManaN != 2 {
		t.Errorf("B B: %+v refusals %v", def.Abilities[0], rs)
	}
	for _, c := range []struct{ name, src, want string }{
		{"coloured fee", head + "A:AB$ Mana | Cost$ W T | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"sacrifice", head + "A:AB$ Mana | Cost$ T Sac<2/Artifact.Other/two other artifacts> | Produced$ B R | SpellDescription$ Sac.\n", "several colours at once from a cost beyond {T}"},
		{"restricted to abilities", head + "A:AB$ Mana | Cost$ T | Produced$ G U | RestrictValid$ Activated | SpellDescription$ Abilities only.\n", "several colours at once under a spend restriction"},
		{"second ability", head + "A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Colourless.\nA:AB$ Mana | Cost$ T | Produced$ W U | SpellDescription$ Pair.\n", "several colours at once beside another mana ability"},
		{"unequal", head + "A:AB$ Mana | Cost$ T | Produced$ W W U | SpellDescription$ Uneven.\n", "unequal counts"},
		{"loyalty", "Name:Prism Probe\nManaCost:2 R G\nTypes:Legendary Planeswalker Probe\nLoyalty:3\nA:AB$ Mana | Cost$ AddCounter<1/LOYALTY> | Planeswalker$ True | Produced$ R G | SpellDescription$ Pair.\n", "loyalty ability adding several colours at once"},
	} {
		_, rs := kwardpowBuild(t, "Prism Probe", c.src)
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		if !strings.Contains(strings.Join(all, "; "), c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, all, c.want)
		}
	}
}
