package oracle

import "testing"

// Felidar Umbra (decision 883 of the recording; dr-mbc-bbaa pair 134): the two
// engines part at Cards[52].AttachedTo, gorge holding Starlit Angel and the
// kernel Battle Angels of Tyr.
func TestMBCFelidarUmbraReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "umbra_attach_to_dead_target")
}
