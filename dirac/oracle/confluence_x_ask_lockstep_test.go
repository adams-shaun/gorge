package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Mana Confluence ({T}, Pay 1 life: Add one mana of any color) at the X ask
// (Blaze's {X}{R}, availXWindow): gorge's cast window census prices the
// paid-life source through paidCostShape (engine/rules/paid_mana_census.go),
// which refuses it when its life would not leave the payer alive -- "no census
// counts a source that kills its payer" -- so a Confluence at 1 life raises
// the largest X by nothing. A painland's or a pain-only source's damage is an
// effect, not a cost, and counts whatever the life (Elves of Deep Shadow).
// Burn drives the payer to 1 life with the Confluence untapped.
// dirac/mana_pain.go painAvailMask is the read to match.
var kconfluenceXDecks = [2][]cardset.Entry{
	{
		{Name: "Mana Confluence", Count: 4}, {Name: "Blaze", Count: 4},
		{Name: "Lightning Bolt", Count: 6}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
	},
	{
		{Name: "Blaze", Count: 4}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Shock", Count: 6}, {Name: "Plains", Count: 8},
		{Name: "Mountain", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Serra Angel", Count: 2}, {Name: "Hill Giant", Count: 4},
	},
}

func TestConfluenceXAskLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "manaconfluence", kconfluenceXDecks, 64, "KCONFLUENCE_X_SEEDS")
}
