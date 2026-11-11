package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A land whose own "whenever CARDNAME becomes tapped" line fires off its own
// mana ability (City of Brass: "it deals 1 damage to you"). The tap for mana
// causes the trigger (CR 603.2, 605.1b): it waits for priority and stacks
// above the spell the payment was for. gorge's planner prices the source as
// last resort at its damage (pay.PaymentPlanSelfDamageTrigger), so a plan
// uses City of Brass only when the Mountains and Forests cannot pay (Serra
// Angel's {W}, Air Elemental's {U}). The expected answer at every decision
// is gorge's.
var kcityBrassDecks = [2][]cardset.Entry{
	{
		{Name: "City of Brass", Count: 6}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
		{Name: "Serra Angel", Count: 4}, {Name: "Air Elemental", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5},
		{Name: "Plains", Count: 15}, {Name: "Mountain", Count: 14},
	},
}

func TestCityOfBrassLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "citybrass", kcityBrassDecks, 200, "KCITYBRASS_SEEDS")
}
