package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The channel lockstep: the five Kamigawa channel lands' hand
// abilities (Eiganjo's 4 damage to an attacker or blocker, Sokenzan's two
// hasty Spirits, Boseiju's destroy-and-search, Otawara's bounce, Takenuma's
// mill-and-return) cost {1} less per legendary creature you control (gorge
// ownReduceCost: the generic part, floored at zero). Vanilla legends on
// both sides make the reduction live; artifacts, an enchantment and the
// lands themselves feed Boseiju and Otawara.
var kchannelDecks = [2][]cardset.Entry{
	{
		{Name: "Eiganjo, Seat of the Empire", Count: 3}, {Name: "Sokenzan, Crucible of Defiance", Count: 3},
		{Name: "Boseiju, Who Endures", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 4},
		{Name: "Happy Hogan, Dauntless Driver", Count: 3}, {Name: "Tyrox, Saurid Tyrant", Count: 3},
		{Name: "Jasmine Boreal", Count: 2}, {Name: "Grizzly Bears", Count: 3}, {Name: "Glorious Anthem", Count: 2},
		{Name: "Ornithopter", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 3},
	},
	{
		{Name: "Otawara, Soaring City", Count: 3}, {Name: "Takenuma, Abandoned Mire", Count: 3},
		{Name: "Kalakscion, Hunger Tyrant", Count: 3}, {Name: "Caelorna, Coral Tyrant", Count: 3},
		{Name: "Wind Drake", Count: 3}, {Name: "Walking Corpse", Count: 4}, {Name: "Typhoid Rats", Count: 3},
		{Name: "Ornithopter", Count: 2}, {Name: "Mind Rot", Count: 2}, {Name: "Counterspell", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
	},
}

func TestChannelLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "channel", kchannelDecks, 1000, "KCHANNEL_SEEDS")
}

// TestChannelLockstepMutation proves the pool prices a reduced channel:
// without the reduction, some seed diverges.
func TestChannelLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoOwnReduce = true
	defer func() { dirac.DebugNoOwnReduce = false }()
	if !lockstepDiverges(t, "channel", kchannelDecks, 40) {
		t.Fatal("no seed diverged without ReduceCost$: the pool never activates a reduced channel")
	}
}
