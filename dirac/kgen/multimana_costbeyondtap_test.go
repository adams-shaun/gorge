package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerMultimanaCostBeyondTap: an all-at-once two-colour mana ability
// whose cost goes beyond {T}. "{1}, {T}: Add {W}{U}" (Skycloud Expanse's
// shape) lowers to a first-only filter the kernel activates through its
// prefix search (kop.FilterEachManaAb, kernel mana_filtereach.go); "{T},
// Sacrifice another Goblin: Add {B}{R}" (Bolg's Company's shape) is refused by
// name: the kernel's prefix search has no sacrifice gap kind. Each other
// out-of-shape variant still refuses by name.
func TestLowerMultimanaCostBeyondTap(t *testing.T) {
	const land = "Name:Expanse Probe\nManaCost:no cost\nTypes:Land\n"
	def, rs := kwardpowBuild(t, "Expanse Probe", land+"A:AB$ Mana | Cost$ 1 T | Produced$ W U | SpellDescription$ Add {W}{U}.\n")
	if len(rs) != 0 {
		t.Fatalf("{1},{T}: Add {W}{U} refused: %v", rs)
	}
	j := -1
	for i := range def.Abilities {
		if def.Abilities[i].Kind == kop.AbMana {
			j = i
			break
		}
	}
	if j < 0 {
		t.Fatal("no mana ability lowered")
	}
	a := &def.Abilities[j]
	if !a.ManaEach || a.ManaColors != kop.ColW|kop.ColU || a.ManaN > 1 || a.Cost.Flags != kop.CostTap || a.Cost.Mana.Generic != 1 {
		t.Fatalf("ability %+v: want ManaEach W|U, {1},{T}", a)
	}
	if !kop.FilterEachManaAb(a) || !kop.FilterManaAb(a) || !kop.ManaAbilityHonoured(&def, j) {
		t.Fatalf("ability %+v is not the honoured all-at-once filter", a)
	}

	const goblin = "Name:Company Probe\nManaCost:B R\nTypes:Creature Goblin Soldier\nPT:2/2\n"
	for _, c := range []struct{ name, src, want string }{
		{"sacrifice another", goblin + "A:AB$ Mana | Cost$ T Sac<1/Goblin.Other/another Goblin> | Produced$ B R | SpellDescription$ Add {B}{R}.\n",
			"from a cost beyond {T}: a sacrifice of another permanent (no sacrifice gap kind"},
		{"coloured fee", land + "A:AB$ Mana | Cost$ W T | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"two of each", land + "A:AB$ Mana | Cost$ 1 T | Produced$ W U W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"fee and sacrifice self", land + "A:AB$ Mana | Cost$ 1 T Sac<1/CARDNAME> | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"restricted", land + "A:AB$ Mana | Cost$ 1 T | Produced$ W U | RestrictValid$ Spell.Creature | SpellDescription$ Filter.\n", "mana restriction on a source whose cost goes beyond {T}"},
		{"beside another", land + "A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ C.\nA:AB$ Mana | Cost$ 1 T | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once beside another mana ability"},
	} {
		name := "Expanse Probe"
		if strings.HasPrefix(c.src, goblin) {
			name = "Company Probe"
		}
		_, rs := kwardpowBuild(t, name, c.src)
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		if !strings.Contains(strings.Join(all, "; "), c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, all, c.want)
		}
	}
}
