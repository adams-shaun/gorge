package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Mastery lockstep: Baleful, Fervent and Ingenious Mastery branch on
// Count$AltCost.1.0, whether the spell was cast for its own "you may pay ...
// rather than pay this spell's mana cost" static (gorge FlagAltCostPaid; the
// kernel's ExAltCost reads the spell entry's trigPlAltCost mark through
// altCostCast, cast_altfree.go takeAltCast). The oracle plays gorge's
// payment-planner surface, which offers the alternative cast only when the
// floating pool pays it, so Dark Ritual and Seething Song float the mana at
// priority. The expected answer at every decision is gorge's.
var kmasteryDecks = [2][]cardset.Entry{
	{
		{Name: "Baleful Mastery", Count: 5}, {Name: "Fervent Mastery", Count: 3}, {Name: "Ingenious Mastery", Count: 3},
		{Name: "Dark Ritual", Count: 6}, {Name: "Seething Song", Count: 4},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 20},
	},
}

func TestMasteryLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "mastery", kmasteryDecks, 400, "KMASTERY_SEEDS")
}
