package oracle

import "testing"

// Goliath Beetle ({2}{G}) was cast-able in gorge but not offered by dirac at
// priority (decision 224, pool tail-73 pair 1, seed 428311; a board holding
// Untaidake, the Cloud Keeper's restricted {C}{C}). Fixed on main since by the
// restricted-gap/filter-walk offer work; the recording stays as the guard.
func TestMBCGoliathBeetleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "goliath_beetle_cast_offered")
}
