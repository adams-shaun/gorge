package oracle

import "testing"

// CR 702.168 (Disguise) with CR 708.6: a face-down Arno Dorian's turn-up cost
// {B}{R} is a potential play when the {R} is a Mercadian Bazaar's idle storage
// production: gorge's potential walk prices that battery open, so it lists the
// turn-up and the kernel must offer the same special action (decision 539 of
// 539, tail-26 pair 1 seed 639944).
func TestMBCArnoDorianReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "arno_dorian_turn_up_storage_battery")
}
