package oracle

import "testing"

// CR 106.1 / 109.3 / 613.1e: Plaza of Heroes' "add one mana of any color among
// legendary permanents you control" reads the permanents' current colours.
// Grave Betrayal returned Phlage, Titan of Fire's Fury as a black Zombie, so
// the Plaza pays {B} for Spined Fluke's regenerate. gorge read only printed
// colours and left the activation out of the priority options (decision 695,
// tail-26-c000 pair 97).
func TestMBCSpinedFlukeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_mana_reads_layered_colors")
}
