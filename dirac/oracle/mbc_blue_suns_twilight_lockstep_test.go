package oracle

import "testing"

// CR 601.2b/601.2c: Blue Sun's Twilight ({X}{U}{U}) is offered only when some
// creature with mana value X or less can be targeted with the X that can be
// paid. Recorded from tail-28 pair 56, decision 211.
func TestMBCBlueSunSTwilightReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "blue_suns_twilight_zero_x_target")
}
