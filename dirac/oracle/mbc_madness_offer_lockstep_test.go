package oracle

import "testing"

// Twins of Maurer Estate (Madness {2}{B}) discarded to Tortured Existence: the
// cast-or-graveyard ask must offer "yes" exactly when gorge does (decision 704 of
// tail-23-c000 pair 3; gorge offered yes and no, the kernel only no).
func TestMBCTwinsOfMaurerEstateReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "madness_cast_offer_affordability")
	// CR 601.2g, 702.35a: the same card discarded with Sand Silos holding no
	// storage counter beside Sunken Ruins and a Mountain: the {B} cannot be
	// made, so the ask offers only "no" (decision 813 of tail-23-c000 pair 38;
	// gorge offered no, the kernel yes and no).
	stdReproNamed(t, "madness_cast_empty_storage_land_unpayable")
}
