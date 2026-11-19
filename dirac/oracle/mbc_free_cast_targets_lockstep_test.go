package oracle

import "testing"

// CR 601.2c/601.2f with CR 118.9: Omnispell Adept casts Revoke Demise without
// paying its mana cost, so the spell's "costs {2} less if it targets a creature
// card with mana value 3 or less" can't decide whether a target is affordable:
// there is no mana cost to pay. Every creature card in its controller's
// graveyard is a legal target, Guardian of Tazeem (mana value 5) and Thousand
// Winds (mana value 6) included. gorge once priced the free cast's targets as if
// the printed cost still applied and offered only the mana value 3 or less
// creatures (decision 641; tail-29-c001 pair 53).
func TestMBCGuardianOfTazeemReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_target_ignores_target_reduction_gate")
}
