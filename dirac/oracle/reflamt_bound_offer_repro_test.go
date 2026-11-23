package oracle

import "testing"

// Wall of Bone's "{B}: Regenerate this creature" (tail-55 pair 137, seed
// 643597) beside a live Incubation Druid (reflected Amount$) and a granted
// mana ability: gorge offers the activation (CR 605.3b: the Druid's mana is
// real, one colour ask, the whole amount), and the kernel must too.
func TestReflAmtBoundActOfferedBesideGrantLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_bound_act_offered_beside_grant")
}
