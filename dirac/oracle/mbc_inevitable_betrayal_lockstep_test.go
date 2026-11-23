package oracle

import "testing"

// CR 702.62a with CR 116.2 and 605.3a: suspend is a special action whose cost
// is paid as it is taken, so its payment may activate mana abilities. Inevitable
// Betrayal ({3}{U}{U}, suspend 3-{1}{U}{U}) beside one Urza, Lord High Artificer
// and three untapped artifacts can pay the suspend cost by tapping each artifact
// for {U} through Urza. gorge offered the suspend; the kernel's offerSpecialHand
// priced Urza as one relaxed unit on the census and withheld it (decision 357 of
// tail-64-c000 pair 55). It now hands the play to the tap-another route, as
// priorityHandAct does for a hand activation (76af2a5dd5).
func TestMBCInevitableBetrayalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "suspend_tap_another_mana_prefix")
}
