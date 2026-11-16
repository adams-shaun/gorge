package oracle

import "testing"

// Lion Sash's {W} exile-a-graveyard-card activation: gorge offers it at
// priority and dirac does not (decision 644 of tail-56 pair 118).
func TestMBCLionSashReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "lion_sash_exile_activation")
}
