package oracle

import "testing"

// A live Incubation Druid (reflected Amount$) leaves gorge's census
// unrelaxable: its census-gap fallback runs the prefix search for every
// census-incomplete play its PotentialMana bound pays (mana_reflamt.go
// reflAmtBoundPlay). On a board with no filter beside the Druid the plays the
// plain census prices nowhere are gorge's any-mana walk's, and gorge's one
// search over the census gaps -- the grant search beside a live granted mana
// ability -- prices each of them (CR 605.3b: the Druid's mana is real, one
// colour ask, the whole amount). The kernel withheld them until the bound arm
// admitted the any walk's bound-paid plays on the filterless board: Wall of
// Bone's "{B}: Regenerate this creature" (tail-55 pair 137, seed 643597) and
// Anoint's {W} cast (pair 78, seed 1699860231136793065).
func TestReflAmtBoundActOfferedBesideGrantLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_bound_act_offered_beside_grant")
}

func TestReflAmtBoundCastOfferedBesideGrantLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_bound_cast_offered_beside_grant")
}
