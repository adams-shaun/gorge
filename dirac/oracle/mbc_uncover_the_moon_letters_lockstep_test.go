package oracle

import "testing"

// Uncover the Moon-Letters' trigger cost is Draw<X/You> with X the mana spent
// on the cast spell. A rebound cast spends none, so X resolves to 0: gorge
// still offers "pay" (a resolved count is payable, however small) beside
// decline, and dirac once offered decline only (tail-59 pair 57, decision
// 310).
func TestMBCUncoverTheMoonLettersReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "uncover_the_moon_letters_zero_draw_cost")
}
