package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCopySpellSlice: the spell-copy forms of the gap-copyspell slice lower
// with no refusal. NonLegendary$ True names the shared RemoveLegendary
// override in B (The Sixth Doctor, Jackal, Double Major, Storm of Saruman,
// The Clone Saga's delayed trigger); Meletis Charlatan's Controller$
// TargetedController sets C&4 on a target copy; Wild Ricochet's Defined$
// Targeted copies its ChangeTargets slot; Chain of Acid's and Chain of Smog's
// Optional$ Parent copy asks target 0's controller.
func TestCopySpellSlice(t *testing.T) {
	nonLeg := []string{"Jackal, Genius Geneticist", "The Sixth Doctor", "Double Major", "Storm of Saruman", "The Clone Saga"}
	names := append([]string{"Meletis Charlatan", "Wild Ricochet", "Chain of Acid", "Chain of Smog"}, nonLeg...)
	cs := lowerNamed(t, names)
	byName := map[string]*lCard{}
	for _, c := range cs {
		byName[c.Name] = c
	}
	copyOf := func(n string) (*lAbility, []lInstr) {
		c := byName[n]
		if c == nil {
			t.Fatalf("%s: not lowered", n)
		}
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", n, r.Reason)
		}
		for _, ab := range c.Abilities {
			for i, in := range ab.Prog {
				if in.Op == "CopySpell" {
					return ab, ab.Prog[:i+1]
				}
			}
		}
		t.Fatalf("%s: no CopySpell", n)
		return nil, nil
	}
	for _, n := range nonLeg {
		_, prog := copyOf(n)
		in := prog[len(prog)-1]
		if !in.B.isConst() || in.B.N < 1 || in.B.N > len(byName[n].overrides) {
			t.Fatalf("%s: CopySpell B=%+v, want the NonLegendary override index+1", n, in.B)
		}
		if o := byName[n].overrides[in.B.N-1]; !o.RemoveLegendary {
			t.Errorf("%s: override %+v, want RemoveLegendary", n, o)
		}
		if in.C&2 == 0 {
			t.Errorf("%s: C=%d, want a permanent-capable copy (C&2)", n, in.C)
		}
	}
	if _, prog := copyOf("Meletis Charlatan"); prog[len(prog)-1].C&4 == 0 || prog[len(prog)-1].Obj.Kind != rTarget {
		t.Errorf("Meletis Charlatan: %+v, want a target copy with C&4", prog[len(prog)-1])
	}
	if ab, prog := copyOf("Wild Ricochet"); prog[len(prog)-1].Obj.Kind != rTarget || !ab.Targets[prog[len(prog)-1].Obj.N].Spell {
		t.Errorf("Wild Ricochet: %+v, want a copy of the spell target", prog[len(prog)-1])
	}
	for _, n := range []string{"Chain of Acid", "Chain of Smog"} {
		_, prog := copyOf(n)
		if len(prog) < 2 {
			t.Fatalf("%s: %+v", n, prog)
		}
		ask, in := prog[len(prog)-2], prog[len(prog)-1]
		if ask.Op != "MayAsk" || ask.C != kop.MayGuardCopyParent || ask.Obj.Kind != rTargetController || in.Obj.Kind != rSelf || in.C&4 == 0 {
			t.Errorf("%s: %+v then %+v, want the target controller's may-copy over a C&4 Parent copy", n, ask, in)
		}
	}
}
