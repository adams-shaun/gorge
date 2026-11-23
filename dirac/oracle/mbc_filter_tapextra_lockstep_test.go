package oracle

import "testing"

// CR 702.74a with CR 605.3b: Cabal Coffers' "{2}, {T}: Add {B} for each
// Swamp you control" is a fee filter (FeeDynManaAb), and its tap is a land
// tapped for mana, so p1's Mana Flare adds one mana of the type the tap
// produced (CR 106.12a: one B beside the two B the two Swamps name). Gorge's
// planner prices the evoked Mournwhelk ({3}{B}) over that production: the
// {2} fee comes off the Plains and the Lens, leaving 3B and one any-colour
// mana. The kernel's filter census (availFilt) once priced the tap without
// the extra (filtExtra asked srcExtraTapped for the pick's colour, and a
// reflect static's fixed vector over a fixed one-colour production was
// dropped as pick-dependent), so its evoked afford read stayed one mana short
// and the "evoked" option never reached the offer.
func TestMBCMournwhelkReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_tapextra")
}
