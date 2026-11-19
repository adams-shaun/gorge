package oracle

import "testing"

// CR 601.2b with CR 601.2c and CR 702.51b: a cast's Convoke announcement runs
// before the targets, so the menu is priced against the pre-target cost -- the
// snapshot that excludes ValidTarget$ cost statics. Caetus, Sea Tyrant of
// Segovia (the transformed Invasion of Segovia) grants Banish from Edoras
// convoke; Banish's own "costs {2} less if it targets a tapped creature"
// reduction is not yet applicable, so the kernel's announcement menu must keep
// offering generic taps after the two picks the game announced instead of
// folding the reduction in early and pricing the cast {2}{W}.
// The recording is on the tail-32 synthetic pool: the tail buckets a synthetic
// pool deals re-cut whenever the lowered card set moves.
func TestMBCBorealElementalReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "boreal_elemental")
}
