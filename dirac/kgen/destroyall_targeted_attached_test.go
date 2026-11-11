package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDestroyAllTargetedAttached: a DestroyAll over the Equipment attached to
// the chain's target (Turn to Slag) lowers to a sweep whose filter reads
// F2AttachedToT0, gorge destroyAllTargetedMatcher's anchor on the target. The
// sweep needs the one object target slot upstream and none of its own; any
// other shape still refuses.
func TestDestroyAllTargetedAttached(t *testing.T) {
	for _, name := range []string{"Turn to Slag", "Blastfire Bolt"} {
		checkTargetedAttachedSweep(t, name)
	}
	checkTargetedAttachedShapes(t)
}

func checkTargetedAttachedSweep(t *testing.T, name string) {
	t.Helper()
	c := lowerCorpusCard(t, name)
	for _, rf := range c.Refusals {
		t.Errorf("%s refused: %s", name, rf.Reason)
	}
	pt := newPoolTables(&lowered{cards: []*lCard{c}}, nil)
	b := buildCard(pt, c)
	if len(b.refusals) != 0 {
		t.Fatalf("%s: build refused: %v", name, b.refusals)
	}
	sweeps := 0
	for _, a := range b.def.Abilities {
		for _, in := range a.Prog {
			if in.Op != kop.OpDestroyAll {
				continue
			}
			sweeps++
			f := pt.filters[in.F]
			if f.Flags2&kop.F2AttachedToT0 == 0 || f.Or != 0 {
				t.Errorf("sweep filter %+v, want the one clause reading F2AttachedToT0", f)
			}
		}
	}
	if sweeps != 1 {
		t.Fatalf("%s: %d DestroyAll instructions, want 1", name, sweeps)
	}
}

func checkTargetedAttachedShapes(t *testing.T) {
	t.Helper()

	// the probe is itself an Equipment so the pool knows the subtype
	const head = "Name:Slag Probe\nManaCost:2\nTypes:Artifact Equipment\n"
	const sweep = "SVar:DBSweep:DB$ DestroyAll | ValidCards$ Targeted.Equipment+Attached\n"
	for _, c := range []struct{ name, src string }{
		{"a player target", head + "A:AB$ DealDamage | ValidTgts$ Player | Cost$ T | NumDmg$ 2 | SubAbility$ DBSweep | SpellDescription$ x.\n" + sweep},
		{"no target at all", head + "A:AB$ Draw | Cost$ T | NumCards$ 1 | SubAbility$ DBSweep | SpellDescription$ x.\n" + sweep},
		{"the sweep's own target", head + "A:AB$ DealDamage | ValidTgts$ Creature | Cost$ T | NumDmg$ 2 | SubAbility$ DBSweep | SpellDescription$ x.\n" +
			"SVar:DBSweep:DB$ DestroyAll | ValidTgts$ Creature | ValidCards$ Targeted.Equipment+Attached\n"},
	} {
		if _, rs := kwardpowBuild(t, "Slag Probe", c.src); len(rs) == 0 {
			t.Errorf("%s: lowered, want a refusal", c.name)
		}
	}
	if _, rs := kwardpowBuild(t, "Slag Probe", head+"A:AB$ DealDamage | ValidTgts$ Creature | Cost$ T | NumDmg$ 2 | SubAbility$ DBSweep | SpellDescription$ x.\n"+sweep); len(rs) != 0 {
		t.Errorf("the one-target shape refused: %v", rs)
	}
}
