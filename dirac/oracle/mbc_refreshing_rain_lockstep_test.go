package oracle

import "testing"

// Refreshing Rain's "if an opponent controls a Swamp and you control a Forest,
// you may cast this spell without paying its mana cost" (decision 384 of pair
// 26, pool tail-62, surface autopay): the kernel offered the free cast and
// gorge did not.
func TestMBCRefreshingRainReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "refreshing_rain_altcost_swamp_forest")
}
