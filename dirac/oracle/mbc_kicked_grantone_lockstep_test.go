package oracle

import "testing"

// CR 106.4 with CR 601.2g: A-Find the Path's "{T}: Add two mana of any one
// color" on a Mountain makes two mana of ONE color. A-Scout the Wilderness
// kicked ({2}{G}{W}) needs a green and a white source, which that Mountain
// and three plain Mountains cannot make, so gorge's planner prices no plan
// and the autopay surface does not list the kicked cast. Dirac's per-tap
// potential census read the grant as two independent any-color mana and
// offered it (decision 304 of 304, ev-tail-62-c000 pair 12).
func TestMBCAScoutTheWildernessReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicked_granted_one_colour_census")
}
