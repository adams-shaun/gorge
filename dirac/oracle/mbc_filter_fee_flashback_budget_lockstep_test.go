package oracle

import "testing"

// CR 601.2f-h, 605.3a, 702.34a: Calibrated Blast flashed back for {3}{R}{R}
// at the upkeep, from a Mountain, three Forests, a Gate to Seatower, Thriving
// Heath and the two "{1}, {T}: Add one mana of any color" filter lands
// (Painted Bluffs, Cave of Temptation). The kernel offered the flashback cast;
// gorge's autopay surface withholds it (census_incomplete), as its prefix
// search over the two fee-bearing gap sources ends past its node budget
// (decision 340, tail-76 pair 13 of the evidence loop's focus deck).
func TestMBCCalibratedBlastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_fee_flashback_search_budget")
}
