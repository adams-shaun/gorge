package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The farm's std-trk lockstep (TRK-s371, game xe:RAND:1:16, explore bots, autopay
// surface) parted from gorge at turn 15 on a creature's power and counter, with Enterprise
// NX-01 implicated: a Spacecraft whose enters trigger taps and stuns an opposing creature
// and which stations (CR 702.184a) into an artifact creature. The pinned game is that
// game: the decks are the farm's, in the decklist order that deals its recorded opening
// hands and libraries at its seed. It reaches Enterprise NX-01 on the battlefield with
// charge counters from a station and must replay clean.
func TestPinnedFarmEnterpriseNX01(t *testing.T) {
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: true}, onBattlefield("Enterprise NX-01"), enterpriseCharged)
}

// enterpriseCharged: Enterprise NX-01 has a charge counter, so it was stationed.
func enterpriseCharged(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Enterprise NX-01" && o.Counter("CHARGE") >= 1 {
				return true
			}
		}
	}
	return false
}
