package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The sunburst lockstep (CR 702.47; gorge kwSunburst's entry replacement):
// Engineered Explosives, Etched Oracle and Suntouched Myr cast from five-colour mana bases enter with a counter per colour spent,
// and Engineered Explosives' sacrifice destroys the nonland permanents whose
// mana value is its charge count. The expected answer at every decision is
// gorge's.
var ksunburstDecks = [2][]cardset.Entry{
	{
		{Name: "Engineered Explosives", Count: 4}, {Name: "Etched Oracle", Count: 3}, {Name: "Suntouched Myr", Count: 3},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Plains", Count: 4}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Engineered Explosives", Count: 2}, {Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

func TestSunburstLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "sunburst", ksunburstDecks, 400, "KSUNBURST_SEEDS")
}
