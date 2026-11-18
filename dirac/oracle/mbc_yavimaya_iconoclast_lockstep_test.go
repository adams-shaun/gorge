package oracle

import "testing"

// CR 702.33a: kicker adds {R} to Yavimaya Iconoclast's {1}{G}. With only a
// Forest and a charge-counter Mirrodin's Core to tap (each one mana), the
// kicked cast's three mana can't be paid, so gorge lists the plain cast only
// (tail-07-c000 pair 78, decision 97).
func TestMBCYavimayaIconoclastReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "yavimaya_iconoclast_kicked_cast_core_counter_mana")
}
