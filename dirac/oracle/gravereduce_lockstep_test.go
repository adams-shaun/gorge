package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The graveyard ReduceCost$ lockstep (CR 601.2f, 602.2b): Razorlash
// Transmogrant's graveyard ability costs {4} less while an opponent controls
// four or more nonbasic lands. The offer and the charge both read the reduced
// cost (CardDef ability OwnReduce, priorityGraveAct). The burn deck kills the
// artifact creature and fields gain lands, the nonbasics the count reads.
var kgraveReduceDecks = [2][]cardset.Entry{
	{
		{Name: "Razorlash Transmogrant", Count: 10}, {Name: "Walking Corpse", Count: 13}, {Name: "Swamp", Count: 17},
	},
	{
		{Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Jungle Hollow", Count: 4}, {Name: "Blossoming Sands", Count: 4}, {Name: "Rugged Highlands", Count: 4},
		{Name: "Wind-Scarred Crag", Count: 4}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 3},
	},
}

func TestGraveReduceLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "gravereduce", kgraveReduceDecks, 300, "KGRAVEREDUCE_SEEDS")
}

// TestGraveReduceLockstepMutation proves the pool activates the reduced
// ability: with the reduction dropped from the kernel, some seed diverges.
func TestGraveReduceLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoOwnReduce = true
	defer func() { dirac.DebugNoOwnReduce = false }()
	if lockstepDiverges(t, "gravereduce", kgraveReduceDecks, 200) {
		return
	}
	t.Fatal("no seed diverged with the reduction dropped: the pool never activates the reduced ability")
}
