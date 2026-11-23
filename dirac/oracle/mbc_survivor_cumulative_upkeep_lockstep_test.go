package oracle

import "testing"

// CR 702.24a: Survivor of the Unseen's cumulative upkeep {2} is paid once per
// age counter. gorge offered the pay option of the upkeep trigger and dirac
// did not (decision 757, pool tail-28-c000 pair 9).
func TestMBCSurvivorOfTheUnseenReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "survivor_of_the_unseen_cumulative_upkeep_pay")
}
