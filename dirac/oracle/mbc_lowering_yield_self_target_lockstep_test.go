package oracle

import "testing"

// CR 715.3d/715.4 with CR 601.2g-h: casting Beanstalk Giant's creature face
// from its adventure zone pays {6}{G} by activating Treasonous Ogre ("Pay 3
// life: Add {R}") and sacrificing three Eldrazi Scions. The Spiteful
// Prankster pings those sacrifices stack must not end the payer before the
// spell is announced. Gorge's canonical vocabulary offers the recast; its
// lowering (payexec) answered the plan-foreign target ask with the positional
// repair clamp's top-up — lowest-index options, the payer's own face at 2
// life — so the pings killed the payer, the lowering dead-ended, and the
// offer was dropped (decision 465 of 465; dr-mbc-bbbb). lowerCompletes now
// retries a positional line that ends with the payer losing, aiming that pick
// at an opposing face; a line the positional answer completes keeps its
// recorded target.
func TestMBCBeanstalkGiantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lowering_yield_self_target")
}
