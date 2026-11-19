package oracle

import "testing"

// Kels, Fight Fixer's "you may pay {U/B}" trigger offered its pay option in
// gorge while the kernel offered only the decline (decision 674, pool tail-41,
// autopay, seed 22429, pair 11).
func TestMBCKelsFightFixerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kels_hybrid_trigger_cost_pay")
}
