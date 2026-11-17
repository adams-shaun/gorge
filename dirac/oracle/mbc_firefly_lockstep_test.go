package oracle

import "testing"

// Firefly's {R} pump activation: Artifact Unknown Shores' {1},{T} any-colour
// filter, funded by a Forest, pays it beside Remote Farm's counter-cost {W}{W}
// source; gorge offers it at priority and dirac does not (decision 539 of
// tail-51 pair 12).
func TestMBCFireflyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_beside_remote_farm_activation")
}
