package oracle

import "testing"

// CR 605.3a, 601.2g: Untaidake, the Cloud Keeper's restricted "{T}, Pay 2 life:
// Add {C}{C}" source is a planner last-resort unit, not a census gap, so it must
// not keep a fee-carrying filter off the filter search. Dirac withheld
// cast(Fatestitcher) that gorge offered (pool tail-73 pair 6, decision 184,
// options options); the same fix as TestMBCGeosurgeReprosLockstep covers it.
func TestMBCFatestitcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "untaidake_restricted_life_source_fatestitcher_cast")
}
