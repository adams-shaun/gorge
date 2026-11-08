package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The farm's std-trk lockstep games that reach U.S.S. Cerritos, Following Up (a
// Spacecraft that stations into an artifact creature, CR 702.184) part from gorge on
// a creature's power. Pinned games replay clean once gorge and the kernel agree.
func TestPinnedFarmUSSCerritosFollowingUp(t *testing.T) {
	if isolate(t) {
		return
	}
	pinnedGames(t, "std-trk", Config{},
		onBattlefield("U.S.S. Cerritos, Following Up"), cerritosCharged, onStack("Test of Conviction"))
}

// cerritosCharged: Cerritos has a charge counter, so it was stationed.
func cerritosCharged(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "U.S.S. Cerritos, Following Up" && o.Counter("CHARGE") >= 6 {
				return true
			}
		}
	}
	return false
}
