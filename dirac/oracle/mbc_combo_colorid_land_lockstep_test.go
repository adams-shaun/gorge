package oracle

import "testing"

// A "Produced$ Combo ColorID" land (Cryptic Spires' circled colours) names no
// colour a game can read, so gorge adds no mana for it (effMana's unhandled
// Produced$ note) and never offers a cast that needs it. The kernel read the
// capital C inside the word "ColorID" as colourless and offered Glacial Wall
// off two Islands and the Spires (decision 96 of tail-00 pair 10, first seen
// under the Gargoyle Sentinel ticket).
func TestMBCGargoyleSentinelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combo_colorid_land_makes_no_mana")
}
