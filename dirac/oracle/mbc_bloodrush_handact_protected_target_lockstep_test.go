package oracle

import "testing"

// Wasteland Viper's bloodrush (decision 667 of pair 7, dr-mbc-aabb seed 1):
// the kernel offered activate(Wasteland Viper) beside pass at the defender's
// priority in the declare-attackers step, against a Blightbeetle with
// protection from green attacking it. The bloodrush ability's source is the
// green card in hand, so CR 702.16b bars it from targeting the beetle and
// gorge correctly offered only pass; the kernel's hand-activation offer ran
// its target feasibility with no source probe (its colours 0), so protection
// from green never matched.
func TestMBCWastelandViperReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bloodrush_handact_protected_target")
}
