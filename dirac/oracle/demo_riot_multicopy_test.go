package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Two or more Uncivil Unrestes on the battlefield: each one's damage doubling
// for a source with a +1/+1 counter is a candidate of the CR 616.1 order ask,
// and the kernel must offer the same candidates as gorge.
var uncivilMultiDecks = [2][]cardset.Entry{
	{
		{Name: "Uncivil Unrest", Count: 4}, {Name: "Rhythm of the Wild", Count: 4},
		{Name: "Spider-Punk", Count: 2}, {Name: "Deadly Recluse", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Glory Seeker", Count: 4}, {Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Counterspell", Count: 4}, {Name: "Uncivil Unrest", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Glory Seeker", Count: 4},
		{Name: "Deadly Recluse", Count: 4}, {Name: "Giant Spider", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Island", Count: 8},
	},
}

// TestUncivilUnrestMultiCopyReplacementOrder: no divergent game over 64 seeds
// with four Uncivil Unrestes in each deck.
func TestUncivilUnrestMultiCopyReplacementOrder(t *testing.T) {
	reg, set, gd := lockstepPool(t, demoRiotPool, uncivilMultiDecks)
	clean, _, d := lockstepSeedsMode(t, reg, set, demoRiotPool, uncivilMultiDecks, gd, 1, 64, lockstepFailEach, nil, nil, castFirstPolicy)
	if len(d) > 0 {
		t.Errorf("%d divergent games", len(d))
	}
	t.Logf("%d clean games", clean)
}
