package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestPinnedFarmQuakestriderCeratops replays the std-fdn farm game that
// diverged on Quakestrider Ceratops (xe:RAND:1:8, turn 18). The first
// difference was the WG seat's Wilt-Leaf Liege discarded by the UB seat's
// Painful Quandary: dirac put it in the graveyard while gorge's
// opponent-caused discard replacement put it onto the battlefield, so the
// Quakestrider lost its +1/+1. The conditions require both permanents to be
// on the battlefield, the state the replacement creates, and the lockstep
// must stay clean.
func TestPinnedFarmQuakestriderCeratops(t *testing.T) {
	permanent := func(name string) func(*GorgeSide) bool {
		return func(g *GorgeSide) bool {
			for p := state.PlayerID(0); p < 2; p++ {
				for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
					if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
						return true
					}
				}
			}
			return false
		}
	}
	rc := RunConfig{Config: Config{GorgeAutopaySurface: true}}
	pinnedGamesRun(t, "std-fdn", rc, 0, permanent("Quakestrider Ceratops"), permanent("Wilt-Leaf Liege"))
}
