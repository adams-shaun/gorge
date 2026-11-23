package oracle

import "testing"

// Goblin Battle Jester and Rhystic Syphon were offered by gorge at a priority
// decision where dirac offered neither (tail-51-c000 pair 13, decision 386).
func TestMBCGoblinBattleJesterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "goblin_battle_jester_offer")
}
