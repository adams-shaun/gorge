package oracle

import "testing"

// CR 605.4 with gorge pay.InstantSpeedOnly: a Lion's Eye Diamond (activate only
// as an instant) is no source for the cost of a trigger that resolves, so a
// "you may pay {1}" ask (Goblinslide) with the player's lands tapped offers
// decline only. dirac once offered pay beside it, counting the Diamond as a
// mana source (decision 120, pool tail-03, autopay).
func TestMBCGoblinslideReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_cost_skips_instant_speed_mana")
}
