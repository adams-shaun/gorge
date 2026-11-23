package oracle

import "testing"

// CR 305.6 and 305.7: Phantasmal Terrain makes Eye of Ugin (a land with no
// mana ability of its own) a Mountain, so it has the intrinsic "{T}: Add {R}"
// and, with a Swamp, pays for Keldon Mantle ({1}{R}) and Sepulcher Ghoul
// ({1}{B}). gorge offered both casts and the kernel only the pass (decision
// 340 of tail-19 pair 21).
func TestMBCKeldonMantleReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "land_type_setter_manaless_land_taps_for_intrinsic")
}
