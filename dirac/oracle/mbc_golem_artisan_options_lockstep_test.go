package oracle

import "testing"

// Golem Artisan is on the battlefield for p1 at decision 495 (a priority
// decision for p1): gorge offers activate(#43,-,0) and activate(#43,-,1) for
// it, the kernel only pass. One side's activation offer is wrong (tail-12
// pool, seed 2254845286384909963, pair 29 of the ev loop).
func TestMBCGolemArtisanActivationOptionsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "golem_artisan_activation_options")
}
