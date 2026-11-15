package oracle

import "testing"

// Dominate ({X}{1}{U}{U}, instant, gain control of target creature with mana
// value X or less) beside Ancient Ziggurat (creature spells only): the kernel
// offers cast(Dominate) at decision 419 where gorge does not (tail-39 pair 54
// at land). The recording plays on the table built from its own decks (pool
// "custom"): a synthetic pool's membership moves with every lowering, and two
// earlier recordings dealt from tail-39 failed the replay on a deck card the
// pool had dropped or gained before they reached the decision.
func TestMBCDominateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dominate_x_spell_offered")
}
