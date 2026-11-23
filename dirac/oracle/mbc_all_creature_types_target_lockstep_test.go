package oracle

import "testing"

// CR 205.3m with CR 702.73a: Blades of Velis Vel gives Akroma, Angel of Fury
// every creature type until end of turn, so she is a legal target for Word of
// Blasting ("destroy target Wall"). Gorge offered the cast; dirac's offer
// builder did not (decision 745 of tail-62-c000, seed 830996, pair 25).
func TestMBCWordOfBlastingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "all_creature_types_grant_matches_subtype_target")
}
