package oracle

import "testing"

// Endless Sands' third ability ({4}, {T}, Sacrifice: return each creature card
// exiled with it) is offered by gorge at decision 427 and not by the kernel
// (tail-42-c000 pair 34).
func TestMBCEndlessSandsReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "endless_sands_return_ability_offered")
}
