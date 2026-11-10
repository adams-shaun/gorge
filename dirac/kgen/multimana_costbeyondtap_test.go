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
// Sacrifice another Goblin: Add {B}{R}" (Bolg's Company's shape) lowers to the
// sacrifice-another source the tap-another search activates
// (kop.SacXEachManaAb, kernel mana_tapanother.go). Each other out-of-shape
// variant (two sacrificed, a fee beside the sacrifice, one colour twice)
// still refuses by name.
func TestLowerMultimanaCostBeyondTap(t *testing.T) {
	const land = "Name:Expanse Probe\nManaCost:no cost\nTypes:Land\n"
	def, rs := kwardpowBuild(t, "Expanse Probe", land+"A:AB$ Mana | Cost$ 1 T | Produced$ W U | SpellDescription$ Probe pair.\n")
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
	sdef, srs := kwardpowBuild(t, "Company Probe", goblin+"A:AB$ Mana | Cost$ T Sac<1/Goblin.Other/another Goblin> | Produced$ B R | SpellDescription$ Probe sac pair.\n")
	if len(srs) != 0 {
		t.Fatalf("{T}, Sacrifice another Goblin: Add {B}{R} refused: %v", srs)
	}
	sj := -1
	for i := range sdef.Abilities {
		if sdef.Abilities[i].Kind == kop.AbMana {
			sj = i
			break
		}
	}
	if sj < 0 {
		t.Fatal("no sacrifice-another mana ability lowered")
	}
	sa := &sdef.Abilities[sj]
	if !sa.ManaEach || sa.ManaColors != kop.ColB|kop.ColR || sa.Cost.Flags != kop.CostTap|kop.CostSac || sa.Cost.N != 1 || sa.Cost.Filter == 0 {
		t.Fatalf("ability %+v: want ManaEach B|R, {T} + sacrifice one", sa)
	}
	if !kop.SacXEachManaAb(sa) || !kop.ManaAbilityHonoured(&sdef, sj) {
		t.Fatalf("ability %+v is not the honoured sacrifice-another source", sa)
	}

	const brush = "Name:Brush Probe\nManaCost:R\nTypes:Creature Goblin\nPT:1/1\n"
	for _, c := range []struct{ name, src, want string }{
		{"sacrifice two others", goblin + "A:AB$ Mana | Cost$ T Sac<2/Goblin.Other/other Goblins> | Produced$ B R | SpellDescription$ Probe sac pair.\n",
			"from a cost beyond {T}: a sacrifice of another permanent (no sacrifice gap kind"},
		{"fee beside the sacrifice", goblin + "A:AB$ Mana | Cost$ 1 T Sac<1/Goblin.Other/another Goblin> | Produced$ B R | SpellDescription$ Probe sac pair.\n",
			"from a cost beyond {T}: a sacrifice of another permanent (no sacrifice gap kind"},
		{"one colour twice", brush + "A:AB$ Mana | Cost$ T Sac<1/Land> | Produced$ R R | SpellDescription$ Probe sac pair.\n",
			"the kernel's mana walk cannot activate it"},
		{"coloured fee", land + "A:AB$ Mana | Cost$ W T | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"two of each", land + "A:AB$ Mana | Cost$ 1 T | Produced$ W U W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"fee and sacrifice self", land + "A:AB$ Mana | Cost$ 1 T Sac<1/CARDNAME> | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once from a cost beyond {T}"},
		{"restricted", land + "A:AB$ Mana | Cost$ 1 T | Produced$ W U | RestrictValid$ Spell.Creature | SpellDescription$ Filter.\n", "mana restriction on a source whose cost goes beyond {T}"},
		{"beside another", land + "A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ C.\nA:AB$ Mana | Cost$ 1 T | Produced$ W U | SpellDescription$ Filter.\n", "several colours at once beside another mana ability"},
	} {
		name := "Expanse Probe"
		if strings.HasPrefix(c.src, goblin) {
			name = "Company Probe"
		} else if strings.HasPrefix(c.src, brush) {
			name = "Brush Probe"
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

// TestFilterEachManaAbNeedsAFee: a plain "{T}: Add {W}{U}" (a bounce land's) is
// no all-at-once filter, so the kernel's prep keeps its all-at-once unit.
func TestFilterEachManaAbNeedsAFee(t *testing.T) {
	const land = "Name:Expanse Probe\nManaCost:no cost\nTypes:Land\n"
	def, rs := kwardpowBuild(t, "Expanse Probe", land+"A:AB$ Mana | Cost$ T | Produced$ W U | SpellDescription$ Probe pair.\n")
	if len(rs) != 0 {
		t.Fatalf("refused: %v", rs)
	}
	a := &def.Abilities[0]
	if a.Kind != kop.AbMana || !a.ManaEach || a.Cost.Mana.Generic != 0 {
		t.Fatalf("ability %+v: want a plain {T} ManaEach", a)
	}
	if kop.FilterEachManaAb(a) || kop.FilterManaAb(a) {
		t.Fatal("a plain {T}: Add {W}{U} reads as a filter")
	}
}
