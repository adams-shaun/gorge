package oracle

import "testing"

// CR 605.3b, 603.3b with CR 702.62a: suspending Infiltrator il-Kor pays its
// {1}{U} with a sacrificed Treasure beside the opponent's Bloodroot Apothecary
// ("whenever an opponent sacrifices a noncreature token, that player gets two
// poison counters"). gorge activates each source at priority before the special
// action, so the sacrifice trigger resolves (poison 2 -> 4) ahead of the
// suspension; the kernel left that trigger on the stack with the poison unmoved
// (decision 380 of 380, dr-tail-53-c000 pair 31).
func TestMBCInfiltratorIlKorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "sacrificed_treasure_special_action_trigger")
}
