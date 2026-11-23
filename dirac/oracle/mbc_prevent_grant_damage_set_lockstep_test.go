package oracle

import "testing"

// CR 616.1: an unblocked 5-power attacker whose damage Benalish Missionary's
// Effect prevents this turn meets Divine Presence ("4 or more damage is 3
// instead") on the next combat's damage event, so gorge asks the affected
// player which replacement applies first. The kernel read the set line and
// the granted prevention in one fixed order and posed no ask (decision 441 of
// 441, tail-35-c000 pair 0 at autopay).
func TestMBCAnaxHardenedInTheForgeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "prevent_grant_beside_damage_set_combat")
}
