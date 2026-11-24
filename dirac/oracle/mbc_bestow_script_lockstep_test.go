package oracle

import "testing"

// CR 702.103b with CR 106.6: a bestowed Nylea's Emissary is cast as an Aura
// spell, so mana that can be spent only on creature spells (Abundant
// Countryside) doesn't pay for it. Gorge's planner returns a script for the
// bestow cast, the script doesn't complete, and the cast isn't offered; dirac
// offered it (decision 298, tail-28 pair 27).
func TestMBCNyleaSEmissaryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bestow_script_dead_end_restricted_mana")
}
