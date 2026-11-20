package oracle

import "testing"

// CR 702.117a with the autopay surface: Grip of the Roil's surge condition
// holds (its controller cast a spell this turn) and its surge cost {1}{U} is
// payable through mana abilities, but gorge's payment planner plans no surged
// cast (kw_surge.go), so under Config.GorgeAutopaySurface gorge lists the
// surged cast only when the floating pool alone pays it. The kernel's
// offerSurgeHand priced it against the cast window's census instead and
// offered it with a lowering witness (decision 221 of 221; dr-mbc-babb pair
// 35, seed 281703), where gorge's list holds no surged cast.
func TestMBCGripOfTheRoilReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "surged_cast_autopay_planner_gate")
}
