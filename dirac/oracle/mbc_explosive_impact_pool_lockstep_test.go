package oracle

import "testing"

// CR 305.6, 605.1a, 605.3b: a land that makes {B} or {G} (Foul Orchard) under
// Urborg's granted Swamp type has the intrinsic "{T}: Add {B}" beside its
// Combo ability (gorge's dedup matches an exact single-colour Produced$ only),
// and gorge's payment planner ranks the intrinsic alternative above the
// printed one, so a generic cost paid from it floats {B}. The kernel folded
// the granted {B} into the Combo's colours and took {G}, leaving the floating
// pool a mana apart (Pool[0][2] / Pool[0][4]).
//
// The second recording is the same grant beside a dual (Plateau, Mountain
// Plains) the synthetic matchup printed no basic types for: the kernel read
// its {R} and {W} as printed and took the granted {B} first, where gorge's
// planner takes its own smallest intrinsic colour {R}. The first fix alone
// swapped that pair's colours between the dual and Luxury Suite.
func TestMBCExplosiveImpactPoolReprosLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range []string{
		"granted_swamp_combo_land_pays_the_granted_colour",
		"granted_swamp_dual_land_keeps_its_intrinsic_order",
	} {
		t.Run(name, func(t *testing.T) { stdReproNamed(t, name) })
	}
}
