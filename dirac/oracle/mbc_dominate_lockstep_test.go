package oracle

import "testing"

// Dominate ({X}{1}{U}{U}, instant, gain control of target creature with mana
// value X or less) at tail-39 pair 24: the kernel offers cast(Dominate) at
// decision 112 where gorge does not.
func TestMBCDominateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dominate_x_spell_offered")
}
