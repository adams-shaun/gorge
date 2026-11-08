package oracle

import "testing"

// TestPinnedFarmReluctantDounguard replays the std-ecl farm game the ECL-s639
// sweep found diverging at turn 15 with Reluctant Dounguard implicated. The
// game is xe:RAND:2:13: its seed is gameSeed(639, 2, 13), and the decks and
// their order are the ones the recorded tape's opening hand and library hold,
// so the pin starts from the farm's exact opening position.
//
// Reluctant Dounguard enters with two -1/-1 counters and, while it has one,
// removes a counter whenever another creature its controller controls enters
// (CR 614.1c, 603.6a). The pinned game puts it on the battlefield and replays
// the whole 17-turn game in lockstep.
func TestPinnedFarmReluctantDounguard(t *testing.T) {
	pinnedGames(t, "std-ecl", Config{GorgeAutopaySurface: true}, onBattlefield("Reluctant Dounguard"))
}
