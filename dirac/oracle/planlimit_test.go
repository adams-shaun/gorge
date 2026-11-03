package oracle

import "testing"

// A game with Leyline of the Guildpact in play, whose payments reach gorge's
// planner node limit: gorge pays with the best plan it had seen, and the
// kernel must stop where gorge stops.
func TestPlanLimitLockstep(t *testing.T) {
	pinnedGames(t, "std-mkm", Config{}, onBattlefield("Leyline of the Guildpact"))
}
