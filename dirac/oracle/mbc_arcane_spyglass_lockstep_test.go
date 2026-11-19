package oracle

import "testing"

// CR 602.2, 605.1a: the recorded game (tail-71-c001, seed 930388) reaches a priority
// decision (476 of 476) where gorge offers Arcane Spyglass's {2}, {T}, Sacrifice a land
// activation beside Horizon of Progress, whose reflected-mana ability ({T}, Pay 1 life)
// funds a generic cost on a board of colourless-only lands. The kernel left the
// activation out (kernel options: pass and Horizon of Progress only) until the life-cost
// reflected land was priced as a {C} pain unit (4ab6c90663). The recording pins that
// both engines now offer it.
func TestMBCArcaneSpyglassReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "arcane_spyglass_sac_land_activation")
}
