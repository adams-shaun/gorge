package oracle

import "testing"

// CR 106.1, 601.2g: gorge's potential walk lists a play only when its bound
// makes the cost's colours, and a reflected Amount$ source (Incubation Druid)
// is folded as {C} alone there. p1's untapped sources are a Swamp and the
// Druid (the tapped Forest lets the Druid reflect {G}, but only at resolution),
// so Vineweft ({G}, an Aura) is no play the walk lists: no search runs for it
// and gorge does not offer it at decision 475. The kernel's wide plain-cast
// arm once searched it and offered it anyway.
func TestMBCVineweftReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "mana_reflected_land_types")
}
