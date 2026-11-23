package oracle

import "testing"

// CR 601.2g: Incubation Druid's reflected Amount$ mana ability is gorge's
// potential bound's {C} alone, so beside a lone Forest the second {G} of
// Viridian Corrupter ({1}{G}{G}) and Scion of Calamity ({3}{G}{G}) is out of
// the bound and gorge runs no prefix search for either cast. The kernel
// searched every plain cast beside the Druid (reflWidePlay) and offered both
// (decision 254, tail-55 pair 45, seed 542963).
func TestMBCScionOfCalamityReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflected_amount_druid_second_green_pip")
}
