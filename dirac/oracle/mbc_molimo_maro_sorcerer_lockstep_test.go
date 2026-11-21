package oracle

import "testing"

// TestMBCMolimoMaroSorcererReprosLockstep replays the synthetic tail-73 game
// the evidence loop recorded (pool tail-73-c000, full surface, seed 697587,
// pair 2): at decision 527 of 527 p0's priority menu held a hand Molimo,
// Maro-Sorcerer that gorge offered (cast(#26{Molimo, Maro-Sorcerer},-,0)) and
// the kernel did not (divergence at decision 527: options options). The
// kernel's offer builders have since agreed with gorge, so this records the
// game as a regression row.
func TestMBCMolimoMaroSorcererReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "molimo_maro_sorcerer_cast_offer")
}
