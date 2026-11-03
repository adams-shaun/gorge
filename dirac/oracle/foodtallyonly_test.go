package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Bonecache Overseer's gate counts a sacrificed Food (three each) through
// PlayerCountPropertyYou$SacrificedThisTurn alone: no filter in this pool
// names the Food subtype, so the matchup keeps it only because the count
// reads it. Bake into a Pie's Food token, sacrificed for its own ability,
// then opens the draw on gorge's side and must open it on the kernel's.
var kfoodtallyonlyDecks = [2][]cardset.Entry{{
	{Name: "Bonecache Overseer", Count: 8}, {Name: "Bake into a Pie", Count: 6}, {Name: "Tragic Slip", Count: 2},
	{Name: "Gravedigger", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 8},
}, {
	{Name: "Bonecache Overseer", Count: 6}, {Name: "Bake into a Pie", Count: 6}, {Name: "Tragic Slip", Count: 4},
	{Name: "Gravedigger", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Swamp", Count: 12}, {Name: "Island", Count: 8},
}}

func TestFoodTallyOnlyLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "foodtallyonly", kfoodtallyonlyDecks, 300, "KFOODTALLYONLY_SEEDS")
}
