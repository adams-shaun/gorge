package oracle

import "testing"

// Bone Sabres' Equip {3} is payable at p0's priority (tail-14-c000 pair 1,
// decision 546, seed 540621, surface autopay): the untapped Swamp makes {B}
// and Jungle Patrol's "Sacrifice a token named Wood: Add {R}" may be
// activated while paying, once per Wood on the battlefield. gorge lists
// activate(Bone Sabres) beside pass where the kernel offers only pass
// (class `options options`).
func TestMBCBoneSabresReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacany_funds_activation_offer")
}
