package oracle

import "testing"

// Extremis Elite's power-up ability ({4}{R}, once; cheaper the turn it entered)
// was offered by gorge at priority but not by the dirac kernel (decision 169 of
// a tail-19 autopay game: gorge [pass, cast(#8)], kernel [pass]), with Eye of
// Ugin and Salt Marsh the enabling lands. The pair replays clean once the
// kernel offers the same activation.
func TestMBCExtremisEliteReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "extremis_elite_powerup_offer_with_eye_of_ugin")
}
