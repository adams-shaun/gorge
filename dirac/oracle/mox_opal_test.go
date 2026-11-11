package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Mox Opal's only mana ability is gated on metalcraft (CR 207.2c, 602.5b):
// three or more artifacts under the controller's control, itself counted. It
// is the only source of white and blue in the first deck, so Serra Angel and
// Air Elemental are castable only while the gate holds; the second deck's
// Shatter closes the gate by destroying an artifact. The expected answer at
// every decision is gorge's.
var kmoxOpalDecks = [2][]cardset.Entry{
	{
		{Name: "Mox Opal", Count: 4}, {Name: "Memnite", Count: 6}, {Name: "Ornithopter", Count: 6},
		{Name: "Mountain", Count: 12}, {Name: "Serra Angel", Count: 4}, {Name: "Air Elemental", Count: 4},
		{Name: "Grizzly Bears", Count: 4},
	},
	{
		{Name: "Shatter", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Grizzly Bears", Count: 5},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 14}, {Name: "Forest", Count: 4},
	},
}

func TestMoxOpalLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "moxopal", kmoxOpalDecks, 200, "KMOXOPAL_SEEDS")
}

// TestMoxOpalGateMutation: the gate treated as always on (the mana-gate
// mutation switch) must diverge from gorge, so the pool proves the metalcraft
// gate is exercised (not only the always-open case). Not parallel: the switch
// is global.
func TestMoxOpalGateMutation(t *testing.T) {
	dirac.SetDebugEntersTappedMutate(5)
	defer dirac.SetDebugEntersTappedMutate(0)
	if !lockstepDiverges(t, "moxopal", kmoxOpalDecks, 200) {
		t.Fatal("gate always on: no divergence in 200 seeds; the pool never closes Mox Opal's gate")
	}
}
