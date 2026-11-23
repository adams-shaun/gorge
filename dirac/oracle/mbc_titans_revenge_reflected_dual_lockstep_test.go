package oracle

import "testing"

// Titan's Revenge ({X}{R}{R}) with Keeper of Progenitus on the table and a
// Sacred Peaks ({T}: {R} or {W}) untapped: the Keeper trigger adds one more
// mana of the type the Peaks produced, so one tap makes {R}{R}, and X = 0 is a
// legal cast (CR 605.3b, CR 107.3). Gorge offered the cast; dirac did not
// (decision 300, tail-59 pair 194).
func TestMBCTitanSRevengeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "titans_revenge_reflected_dual_cast_offer")
}
