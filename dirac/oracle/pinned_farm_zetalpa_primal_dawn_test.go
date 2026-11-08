package oracle

import "testing"

// TestPinnedFarmZetalpaPrimalDawn replays the std-fdn farm game that diverged
// on Zetalpa, Primal Dawn (xe:RAND:4:1, turn 28). The first difference was a
// Deathtouched-marked blocker the attacker's second-strike trample damage
// assigned to again: gorge treats the mark as already-lethal (CR 704.5h,
// 702.19b) and tramples over, while dirac assigned another point. The pinned
// game reaches Zetalpa on the battlefield and must replay clean.
func TestPinnedFarmZetalpaPrimalDawn(t *testing.T) {
	t.Parallel()
	pinnedGamesRun(t, "std-fdn", RunConfig{Config: Config{GorgeAutopaySurface: true}}, 0, onBattlefield("Zetalpa, Primal Dawn"))
}
