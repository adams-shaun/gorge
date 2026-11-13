package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The farm's std-trk lockstep (TRK-s243, game xe:RAND:4:2, explore bots, autopay
// surface) parted from gorge at turn 26 on a token's zone, with Savvy Profiteer
// implicated (its Treasure activation was the latest action). The first difference
// was Replicator's copy of an Officer token made while Kira Nerys, Firebrand's
// controller had an attacker: Kira's replacement (CR 614.1c) moves the copy onto the
// battlefield tapped and attacking, which dirac did, but gorge's mover took the
// not-yet-entered mint for a token that had left the battlefield (CR 111.8) and lost
// it. The pinned game is that game, with the farm's decks in the decklist order that
// deals its recorded hands at its seed. It reaches Savvy Profiteer on the
// battlefield and a token copy entering attacking, and must replay clean.
func TestPinnedFarmSavvyProfiteer(t *testing.T) {
	if isolate(t) {
		return
	}
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: true}, onBattlefield("Savvy Profiteer"), copyTokenAttacking)
}

// copyTokenAttacking: a token copy is on the battlefield attacking.
func copyTokenAttacking(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.IsToken && o.IsCopy && o.IsAttacking {
				return true
			}
		}
	}
	return false
}
