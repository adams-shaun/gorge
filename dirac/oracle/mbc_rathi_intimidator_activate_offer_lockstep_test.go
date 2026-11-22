package oracle

import "testing"

// Rathi Intimidator's "{2}, {T}: search" (CR 602.2, 605.3b): at decision 495 of
// the tail-45-c000 evidence-loop game (pair 8, seed 160867, autopay) p1 held a
// Swamp, a Sandstone Needle ({T}, remove a depletion counter: {R}{R}) and a Food
// token that Hazel's Brewmaster gave a restricted mana ability. gorge offered
// the activation (its prefix search taps the Needle); the kernel's relaxed
// proof dropped the refused grant without counting the Needle's production
// and withheld it.
func TestMBCRathiIntimidatorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rathi_intimidator_priority_activate_offer")
}
