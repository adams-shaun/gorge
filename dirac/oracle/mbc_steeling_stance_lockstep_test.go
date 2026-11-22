package oracle

import "testing"

// TestMBCSteelingStanceReprosLockstep replays the synthetic tail-15 game where p0
// holds Steeling Stance with Plains, Island and Wooded Bastion untapped. The
// filter land turns Plains' {W} into {W}{W} (CR 605.3b), so {1}{W}{W} is payable
// and gorge offers the cast.
func TestMBCSteelingStanceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "forecast_filter_land_cast_offer")
}
