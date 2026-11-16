package oracle

import "testing"

// No One Left Behind costs {3} less "if it targets a creature card with mana
// value 3 or less", and the target is a card in the caster's graveyard. gorge
// reads that ValidTarget$ against any chosen object (CR 601.2f); the kernel
// matched only battlefield permanents, so with {B}{B}{B} floating gorge offered
// the {1}{B} cast and the kernel offered a bare pass (tail-23 pair 181,
// decision 878 of the recording).
func TestMBCNoOneLeftBehindReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "reduce_cost_graveyard_target")
}
