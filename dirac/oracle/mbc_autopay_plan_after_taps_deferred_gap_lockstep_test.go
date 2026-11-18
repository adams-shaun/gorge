package oracle

import "testing"

// Pair 5 of the synthetic pool tail-34 (autopay surface, one mulligan) once
// parted at decision 365 with `kernel: autopay found no plan`: dirac's mana
// step asked its planner to pay a cost the land set could not fund while
// Vault of the Archangel was among the sources (gorge pays it). The recording
// was shrunk at land af2515c069, where it panics, and replays clean at the
// head; the mana-source changes between them (a chosen-colour land is a
// Taps-deferred script gap, CR 603.3) closed it.
func TestMBCVaultOfTheArchangelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "autopay_plan_after_taps_deferred_gap")
}
