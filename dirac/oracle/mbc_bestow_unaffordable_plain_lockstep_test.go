package oracle

import "testing"

// CR 702.103a: a card with bestow may be cast for its bestow cost whether or
// not its mana cost is payable. Trickster's Elk ({2}{G}, bestow {1}{G}) with
// only a Forest and a Swamp to tap is castable bestowed onto Elvish Reclaimer.
// gorge offered cast(Elk, bestowed); the kernel's hand walk left the card at
// the plain cast's affordability gate and never reached offerBestowHand
// (decision 92, pool tail-62-c006).
func TestMBCTricksterSElkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_offered_when_plain_cost_unaffordable")
}
