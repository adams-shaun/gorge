package oracle

import "testing"

// CR 305.7 with CR 613.1d/f: Harbinger of the Seas makes nonbasic lands Islands, so
// Crumbling Necropolis loses its own ability and taps for {U} through the Island type,
// while Dune Chanter grants every land "{T}: Add one mana of any color". gorge's
// planner census holds the intrinsic {U} unit (printed and intrinsic abilities, never
// grants), so its plan pays Shape Anew with the Necropolis beside two Forests and an
// Island and the autopay priority list offers the cast (decision 482, tail-71 pair 23).
// The kernel's ungranted census skipped the land as granted-only and left the cast out.
func TestMBCShapeAnewReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "ungranted_census_land_type_intrinsic")
}
