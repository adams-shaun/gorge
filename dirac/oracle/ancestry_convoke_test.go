package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A convoke cast beside a Path of Ancestry that makes no mana (no commander,
// CR 903.4f): the red pip the announced picks left owed has no source, so the
// target menu is withheld and the cast reverses at the target ask, as the
// kernel does. Gorge's window census must call that source priced (it makes
// nothing), not a gap that keeps the historical offer. The expected answer at
// every decision is gorge's.
var kancestryConvokeDecks = [2][]cardset.Entry{
	{
		{Name: "Path of Ancestry", Count: 6}, {Name: "Stoke the Flames", Count: 8}, {Name: "Goblin Piker", Count: 8},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 6}, {Name: "Plains", Count: 8},
	},
	{
		{Name: "Path of Ancestry", Count: 4}, {Name: "Stoke the Flames", Count: 6}, {Name: "Goblin Piker", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

func TestAncestryConvokeLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ancestryconvoke", kancestryConvokeDecks, 400, "KANCESTRYCONVOKE_SEEDS")
}
