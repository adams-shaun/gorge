package oracle

import "testing"

// Ruthless Deathfang's counter ability must leave the same projected
// Power/Toughness on both engines. The recorded lockstep game diverged at
// decision 327: projection Cards[21].Power (kernel Power 3, gorge Power 4,
// with a Counters[1] counter on the kernel side only).
func TestMBCRuthlessDeathfangReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ruthless_deathfang_counter_power")
}
