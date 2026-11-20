package oracle

import "testing"

// CR 601.2g-h with CR 702.14a: casting Beanstalk Giant's creature face from
// its adventure zone pays {6}{G} by activating Treasonous Ogre ("Pay 3 life:
// Add {R}") and sacrificing three Eldrazi Scions, and the Spiteful Prankster
// pings those sacrifices stack do not end the seat before the spell is
// announced — the seat aims each ping at the opponent (R1: the targeting
// policy never aims the deciding seat's own face while an opposing option
// exists). The kernel offers the recast; gorge's canonical vocabulary must
// offer the same play, and its lowering (payexec) answered a plan-foreign
// target ask by the positional repair clamp's top-up — lowest-index options,
// the payer's own face at 2 life — so the pings killed the payer before the
// cast and the play was dropped as a lower dead end (decision 465 of 465;
// dr-mbc-bbbb).
func TestMBCBeanstalkGiantReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lowering_yield_self_target")
}
