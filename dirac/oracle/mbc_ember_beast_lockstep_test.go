package oracle

import "testing"

// CR 508.1d / CR 506.5: Ember Beast can't attack alone. The attackers
// decision of a recorded tail-03 game (pair 194, decision 659) offers the
// Beast to the kernel but not to gorge.
func TestMBCEmberBeastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ember_beast_attacks_without_partner")
}
