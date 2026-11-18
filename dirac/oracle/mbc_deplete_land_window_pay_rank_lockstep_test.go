package oracle

import "testing"

// A cast paid with Treasonous Ogre ("Pay 3 life: Add {R}") and basic lands on a
// board that also holds Peat Bog ("{T}, Remove a depletion counter: Add {B}{B}"):
// gorge's planner pays the tap sources it ranks first and the Ogre's life unit,
// while the kernel's payment window admitted the bounded Deplete rider ahead of
// them and paid through the Bog (pair 8, pool dr-mbc-bbbb, surface autopay).
func TestMBCDepleteLandWindowPayRankLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "deplete_land_window_pay_rank")
}
