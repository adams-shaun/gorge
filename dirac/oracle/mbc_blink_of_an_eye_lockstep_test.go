package oracle

import "testing"

// Blink of an Eye's kicker: gorge and dirac must offer the same cast options
// (kicked and unkicked) at the priority decision of the recorded game.
func TestMBCBlinkOfAnEyeKickerOfferReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blink_of_an_eye_kicked_offer")
}
