package oracle

import (
	"testing"
)

// CR 701.39a with CR 613.1d: a land animated into a creature by Blinkmoth
// Nexus's own ability is a creature for bolster's "creatures you control",
// so its 1/1 ties for the least toughness and the bolster must offer it.
// Gorge's election read the printed face and left it out while dirac offered
// it (decision 943 of the recording).
func TestMBCBlinkmothNexusReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "bolster_animated_land")
}
