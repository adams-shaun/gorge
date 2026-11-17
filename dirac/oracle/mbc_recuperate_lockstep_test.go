package oracle

import "testing"

// CR 400.7, 601.2a: Dead Man's Chest's Effect grants a permission to cast the
// exiled nonland cards (Duration$ Permanent, Card.nonLand+IsRemembered, no
// ForgetOnMoved$) while they remain exiled. The kernel once lowered that
// static as an emblem filtered on the remembered set, which is gone after the
// Effect's cleanup, so it never offered Recuperate (an instant) from exile
// while gorge did (decision 432, tail-06 pair 0).
func TestMBCRecuperateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mayplay_remembered_nonland_no_forget")
}
