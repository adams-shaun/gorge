package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The any-colour first ability of a counter-cost source (Weather Maker's
// "{T}: Add one mana of any color" beside "{T}, remove two charge counters:
// Add {C}{C}") beside an each-colour gap (Bloom Tender), the board whose
// prefix search is the counter-cost chain (ctrScript): the chain answers the
// first ability's colour ask and floats one mana of that colour, so Hill
// Giant's {3}{R} is not payable once the lands are tapped (the Tender makes
// green, the counter ability colourless) -- a chain step that floated every
// colour of the first ability would pay it.
var kvividFirstDecks = [2][]cardset.Entry{
	{
		{Name: "Weather Maker", Count: 8}, {Name: "Bloom Tender", Count: 6}, {Name: "Hill Giant", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Wind Drake", Count: 8}, {Name: "Lightning Bolt", Count: 8},
		{Name: "Doom Blade", Count: 4}, {Name: "Serra Angel", Count: 4}, {Name: "Mountain", Count: 8},
	},
}

// TestVividCounterFirstAbilityColourLockstep: random and explore policies over
// Weather Maker beside Bloom Tender, no divergence and no unmodelled stall.
func TestVividCounterFirstAbilityColourLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	kvividRun(t, kvividFirstDecks, pkPolicy, 160, nil, false)
	kvividRun(t, kvividFirstDecks, ExplorePolicy, 160, nil, false)
}

// TestVividCounterFirstAbilityColourMutation: with every counter-cost chain
// step reading the first ability as one answer that floats all its colours
// (dirac.DebugCtrFirstAllColours) the lockstep diverges, so the test above
// depends on the colour ask and is not vacuous.
func TestVividCounterFirstAbilityColourMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrFirstAllColours = true
	defer func() { dirac.DebugCtrFirstAllColours = false }()
	if kvividRun(t, kvividFirstDecks, pkPolicy, 160, nil, true) == nil && kvividRun(t, kvividFirstDecks, ExplorePolicy, 160, nil, true) == nil {
		t.Error("no divergence with the first ability floating every colour")
	}
}
