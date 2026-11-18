package oracle

import "testing"

// Increasing Savagery's flashback ({7}{G}{G}) was offered by the kernel as a
// cast from the graveyard where gorge offered none (decision 357 of 357,
// tail-53-c000 pair 23).
func TestMBCIncreasingSavageryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "flashback_cast_offered_unaffordable")
}
