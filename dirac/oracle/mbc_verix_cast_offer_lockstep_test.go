package oracle

import "testing"

// Verix Bladewing's cast was offered by dirac alone at priority (decision 662 of
// tail-55-c000 pair 95, autopay, seed 303273, main 93b4a5b399): the kernel listed
// cast(#38{Verix Bladewing}) beside the plays gorge offered and gorge did not.
// The recorded game replays whole and clean on both engines now.
func TestMBCVerixBladewingReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "verix_bladewing_cast_offered_only_by_kernel")
}
