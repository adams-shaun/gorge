package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Animate "target land becomes a <basic land type>" shapes in lockstep over
// the animateshapes fixture pool: RemoveLandTypes$ True beside a literal
// Types$ is the CR 305.7 land-type setter record (kop.Override.SetLandSub, the
// shape Dream Thrush's chosen type writes) in every duration the cards use
// (until end of turn, Permanent, UntilHostLeavesPlay, AsLongAsInPlay), from an
// activated ability, a dies trigger, a combat-damage trigger and a kicked
// enters trigger. Each deck holds colours only a set land can pay for.
var animateLandTypeDecks = [2][]cardset.Entry{
	{
		{Name: "Floodchaser", Count: 3}, {Name: "Kavu Recluse", Count: 3}, {Name: "Cyclopean Giant", Count: 3},
		{Name: "Tide Shaper", Count: 3}, {Name: "Deepwood Elder", Count: 3}, {Name: "Tidal Warrior", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Island", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 3},
	},
	{
		{Name: "Zombie Trailblazer", Count: 3}, {Name: "Slimy Kavu", Count: 3}, {Name: "Tundra Kavu", Count: 3},
		{Name: "Graceful Antelope", Count: 3}, {Name: "Thelonite Monk", Count: 3},
		{Name: "Dreamwinder", Count: 2}, {Name: "Streambed Aquitects", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Island", Count: 5}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4},
	},
}

// TestAnimateLandTypeLowers: the pool plays in lockstep.
func TestAnimateLandTypeLowers(t *testing.T) {
	t.Parallel()
	lockstepRunPolicy(t, "animateshapes", animateLandTypeDecks, 200, true, nil, nil, func(seed uint64) Policy { return castFirstPolicy(seed) })
}
