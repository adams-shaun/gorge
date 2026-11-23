package oracle

import "testing"

// Recorded as tail-55 pair 45 (surface full, mulligans 1, seed 542963): after the
// reflected-Amount$ plain-cast fix the pair parts at a priority decision where
// gorge lists a {G} cast beside an untapped Incubation Druid (every land tapped)
// that the kernel does not offer.
func TestMBCIncubationDruidAdaptReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reflamt_druid_green_cast_offer_beside_tapped_forest")
}
