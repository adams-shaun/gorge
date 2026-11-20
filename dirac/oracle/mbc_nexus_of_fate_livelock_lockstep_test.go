package oracle

import "testing"

// Nexus of Fate's extra turn repeated across turns while the same lands
// untapped and the same priority decisions were asked again: gorge's livelock
// guard fired at decision 1193 on a cycle of 121 events. The recording pins
// the game where the extra-turn loop never advances.
func TestMBCNexusOfFateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "nexus_of_fate_livelock")
}
