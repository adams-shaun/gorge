package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The chosen-source shield's recipient and lifetime variants
// (kop.PrevNextChosenTo/Any/Turn, dirac/repl_prevchosen.go), played decision
// by decision against gorge's effEffect DamageDone replacement: the shield on
// the Effect's own target (Martyr's Cause and Sanctum Guardian over
// RememberObjects$ Targeted, Charm Peddler and Samite Blessing over
// ImprintCards$ Targeted), on any recipient (Pilgrim of Justice, Pilgrim of
// Virtue), and the bodyless Prevent$ True line that prevents every event from
// the source this turn (Pay No Heed, Auriok Replica, Burrenton Forge-Tender).
var kchosenVariantDecks = [2][]cardset.Entry{
	{
		{Name: "Pay No Heed", Count: 3}, {Name: "Auriok Replica", Count: 3}, {Name: "Martyr's Cause", Count: 2},
		{Name: "Sanctum Guardian", Count: 2}, {Name: "Pilgrim of Justice", Count: 2}, {Name: "Pilgrim of Virtue", Count: 2},
		{Name: "Burrenton Forge-Tender", Count: 3}, {Name: "Charm Peddler", Count: 2}, {Name: "Samite Blessing", Count: 2},
		{Name: "Savannah Lions", Count: 3}, {Name: "Plains", Count: 16},
	},
	{
		{Name: "Hill Giant", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Dusk Imp", Count: 4},
		{Name: "Shock", Count: 5}, {Name: "Lightning Bolt", Count: 5}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 8},
	},
}

func TestChooseSourceVariantsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "chosenshields", kchosenVariantDecks, 200, "KCHOSENSHIELDS_SEEDS")
}

// TestChooseSourceVariantsMutation: with the shields installed but preventing
// nothing, some seed diverges, so the variants do prevent damage.
func TestChooseSourceVariantsMutation(t *testing.T) {
	dirac.DebugNoPrevChosen = true
	defer func() { dirac.DebugNoPrevChosen = false }()
	if !lockstepDiverges(t, "chosenshields", kchosenVariantDecks, 200) {
		t.Fatal("no seed diverged with the chosen-source shields disabled")
	}
}
