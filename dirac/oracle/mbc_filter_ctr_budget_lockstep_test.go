package oracle

import "testing"

// A Mystic Gate-style {W/U} filter land beside a Vivid Meadow-style counter-cost
// any-colour land and a colourless land: gorge's prefix search (160 nodes) runs
// out before it finds the line that casts Slitherwisp ({U}{B}{B}), so gorge
// offers no cast; the kernel's mirror must run out of the same budget.
func TestMBCSlitherwispReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "filter_ctr_gap_script_budget")
}
