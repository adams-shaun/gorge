package oracle

import "testing"

// The std-fdn farm game that diverged with Mystic Archaeologist implicated
// (options at turn 20) is pinned on the full surface: games of the synthetic
// pool that reach Mystic Archaeologist on the battlefield play clean in lockstep.
// The divergence itself (Pyromancer's Goggles' deferred {R} beside New Horizons'
// granted mana in the prefix search) is the deferred_gap_beside_granted_mana
// repro of TestStdFdnReprosLockstep.
func TestPinnedFarmMysticArchaeologist(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-fdn", Config{}, onBattlefield("Mystic Archaeologist"))
}
