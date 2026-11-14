package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The MBC protection-from-mana-value lockstep: Mistmeadow Skulk (protection
// from mana value 3 or greater) and Reaver Titan (3 or less) on seat 0
// against seat 1's burn, Torch Breath among it. An X spell's mana value
// carries its announced X (CR 202.3e), which CR 601.2b announces before the
// targets of 601.2c, so Torch Breath cast for X = 2 may not target the Skulk
// and a resolving one reads the same X. The expected answer at every
// decision is gorge's. Without the lowering the pool's table refuses both
// cards and the matchup cannot be built.
var kmbcprotectionmvDecks = [2][]cardset.Entry{
	{
		{Name: "Mistmeadow Skulk", Count: 8}, {Name: "Reaver Titan", Count: 2},
		{Name: "Savannah Lions", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Plains", Count: 24},
	},
	{
		{Name: "Torch Breath", Count: 10}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 18},
	},
}

func TestMBCProtectionManaValueLowers(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mbcprotectionmv", kmbcprotectionmvDecks, 400, "KMBCPROTECTIONMV_SEEDS")
}
