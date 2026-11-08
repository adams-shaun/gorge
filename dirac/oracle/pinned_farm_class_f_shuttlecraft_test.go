package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The farm's std-trk lockstep (TRK-s278, game xe:RAND:0:6, explore bots, autopay surface,
// seat 0 the tape's p1 deck) parted from gorge at decision 471, turn 18, on a creature's
// power, toughness and counter, with Class-F Shuttlecraft implicated: a Spacecraft that
// stations (CR 702.184a) into a creature at 6+ charge counters. The first difference was
// not the Spacecraft's: a +1/+1 counter that dirac put on a permanent entering, and gorge's
// entry preview did not give it before a lent entry rider was matched (CR 614.12, 122.6);
// 31096a095 fixed that in gorge. The pinned game is that game: the decks are the farm's, in
// the decklist order that deals the tape's recorded opening hands and libraries at the seed
// 278*1000003+6. It reaches a stationed Class-F Shuttlecraft and must replay clean.
func TestPinnedFarmClassFShuttlecraft(t *testing.T) {
	pinnedGames(t, "std-trk", Config{GorgeAutopaySurface: true}, onBattlefield("Class-F Shuttlecraft"), classFCharged)
}

// classFCharged: a Class-F Shuttlecraft holds at least six charge counters, the station
// threshold at which it is a creature.
func classFCharged(ref *GorgeSide) bool {
	g := ref.E.G
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Class-F Shuttlecraft" && o.Counter("CHARGE") >= 6 {
				return true
			}
		}
	}
	return false
}
