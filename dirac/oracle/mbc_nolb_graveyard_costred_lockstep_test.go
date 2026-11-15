package oracle

import "testing"

// No One Left Behind ({4}{B}, "this spell costs {3} less to cast if it
// targets a creature card with mana value 3 or less"): at decision 160 of
// tail-47 pair 10 (seed 5628159775108598851) gorge offered the cast whose
// reduction the graveyard target satisfies; the kernel priced the full cost
// and offered none.
func TestMBCNoOneLeftBehindReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "nolb_graveyard_costred")
}
