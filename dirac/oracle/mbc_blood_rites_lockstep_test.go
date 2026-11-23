package oracle

import "testing"

// Pool[1][0] (generic-capable mana in seat 1's pool) parted at decision 990
// of the tail-15-c000 autopay game after Multiform Wonder was cast.
func TestMBCBloodRitesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blood_rites_pool_projection")
}
