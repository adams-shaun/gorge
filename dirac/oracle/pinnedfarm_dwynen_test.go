package oracle

import (
	"testing"
)

// TestPinnedFarmDwynenGiltLeafDaen replays the std-fdn farm game that diverged
// with Dwynen, Gilt-Leaf Daen implicated (FDN-s397, xe:RAND:3:1, turn 14). The
// WG seat's Wilt-Leaf Liege was discarded by the opposing UB seat's Painful
// Quandary; the discard replacement (CR 614.1) puts the Liege onto the
// battlefield in gorge while dirac put it into the graveyard, so the green and
// white creatures, Dwynen among them, lost its +1/+1. The recorded decks are
// recovered from the farm game's tape (seed 403031246 reproduces its opening
// exactly); the actions are a fixed prefix from the seeded explore policy that
// reaches Dwynen on the battlefield and the discard replacement. Both must
// hold, and the lockstep must stay clean.
func TestPinnedFarmDwynenGiltLeafDaen(t *testing.T) {
	t.Parallel()
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-fdn", rc, 0,
		onBattlefield("Dwynen, Gilt-Leaf Daen"), discardedIntoPlay("Wilt-Leaf Liege"))
}
