package oracle

import "testing"

// Punish the Enemy (two targets: a player or planeswalker, then a creature):
// gorge offered the cast at priority and dirac did not (decision 346 of 346,
// tail-39 pair 887 under autopay).
func TestMBCPunishTheEnemyReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "punish_the_enemy_options")
}
