package oracle

import "testing"

// CR 205.3m, 702.73a, 205.1b: Muzzio wears Coalition Flag, so it is a Flagbearer
// and an opponent choosing targets must choose a Flagbearer if able. Their
// 2/2 Shapeshifter tokens carry changeling (every creature type, Flagbearer
// included), so they are legal answers beside Muzzio. Gorge read the
// requirement through the derived type list alone, which never holds
// changeling's types, and offered Advance Scout's target ask only Muzzio
// (decision 620, tail-12-c000 pair 6).
func TestMBCAdvanceScoutReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "advance_scout_flagbearer_changeling")
}
