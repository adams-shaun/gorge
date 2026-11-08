package oracle

import "testing"

// The farm's std-ecl lockstep (ECL-s247 game 11) ended in a kernel panic, "payment
// failed (pips)", with Dawn's Light Archer implicated: a {2}{G} Flash creature whose
// cast paid from a pool another earlier payment had left different. The pinned games
// reach the Archer on the battlefield and must replay clean.
func TestPinnedFarmDawnsLightArcher(t *testing.T) {
	t.Parallel()
	pinnedGames(t, "std-ecl", Config{GorgeAutopaySurface: true}, onBattlefield("Dawn's Light Archer"))
}
