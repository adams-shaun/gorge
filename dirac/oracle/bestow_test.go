package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The bestow lockstep: Nyxborn Hydra cast bestowed (gorge's
// "bestowed" option, X G G) is an Aura spell with enchant creature, so
// Essence Scatter cannot counter it and Negate can; it resolves attached
// (an Aura, not a creature, granting +1/+1 per +1/+1 counter, reach and
// trample), or as a creature when Shock or Unsummon took its target first
// (CR 702.103e), and it detaches and stays a creature when its host leaves
// (CR 702.103f).
var kbestowDecks = [2][]cardset.Entry{
	{
		{Name: "Nyxborn Hydra", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Giant Growth", Count: 3}, {Name: "Centaur Courser", Count: 4}, {Name: "Runeclaw Bear", Count: 4},
		{Name: "Forest", Count: 17},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Essence Scatter", Count: 3}, {Name: "Negate", Count: 3},
		{Name: "Unsummon", Count: 3}, {Name: "Wind Drake", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func TestBestowLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "bestow", kbestowDecks, 1000, "KBESTOW_SEEDS")
}

// TestBestowLockstepMutation proves the pool casts bestowed: without the
// bestowed option, some seed diverges.
func TestBestowLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoBestow = true
	defer func() { dirac.DebugNoBestow = false }()
	if !lockstepDiverges(t, "bestow", kbestowDecks, 40) {
		t.Fatal("no seed diverged without the bestowed cast: the pool never bestows")
	}
}
