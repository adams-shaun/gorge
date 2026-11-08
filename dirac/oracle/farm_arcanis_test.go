package oracle

import "testing"

// The std-fdn farm game that diverged with Arcanis the Omnipotent implicated
// (options at turn 28) is pinned on the full surface: games of the synthetic
// pool that reach Arcanis the Omnipotent on the battlefield play clean in lockstep.
func TestPinnedFarmArcanistheOmnipotent(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-fdn", Config{}, onBattlefield("Arcanis the Omnipotent"))
}
