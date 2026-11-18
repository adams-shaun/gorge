package oracle

import "testing"

// CR 107.4f, 601.2b, 601.2g: Thundering Tanadon ({4}{G/P}{G/P}) at two life
// beside four lands and Saruli Caretaker ("{T}, Tap an untapped creature you
// control: Add one mana of any color"). gorge's planner prices only the cost's
// pip-free half ({4}, pay.PlanManaHalf), so the cast is a planner-priced one
// the autopay surface keeps, and its prefix search finds the Caretaker's mana
// for the fifth ({4}{G} with one pip paid with two life). The kernel dropped
// the plain cast from the scripted offers as one the planner cannot price
// (decision 1044, tail-28-c000 pair 75, seed 548951).
func TestMBCThunderingTanadonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phyrexian_cast_half_priced_beside_tapx_gap")
}
