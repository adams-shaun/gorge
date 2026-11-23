package oracle

import "testing"

// CR 605.1a / 601.2g: Incubation Druid's reflected, Amount$-scaled mana
// ability lets Forest + Druid pay Viridian Corrupter's {1}{G}{G}. Replays
// tail-55 pair 35, where the kernel offered the cast at decision 185 and gorge
// did not.
func TestMBCViridianCorrupterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "viridian_corrupter_reflamt_search")
}
