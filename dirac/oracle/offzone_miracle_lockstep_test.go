package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted hand miracle lockstep (CR 702.94a): a battlefield static
// granting Miracle to cards in a hand (Lorehold, the Historian's instants and
// sorceries at {2}, Molecule Man's nonland cards at {0}, Aminatou, Veil
// Piercer's enchantments at their mana cost reduced by {4}) gives the kernel
// the same first-draw reveal ask, the same resolution ask and the same cast
// at the granted cost that gorge's derived-keyword walk gives.
//
// The pool is the off-battlefield statics fixture (offzonegrants). Each
// deck runs a grantor over cards its filter reaches, so the first draw of a
// turn (the draw step) reveals.
var offZoneMiracleDecks = [2][]cardset.Entry{
	{
		{Name: "Lorehold, the Historian", Count: 6}, {Name: "Lightning Bolt", Count: 8}, {Name: "Giant Growth", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 8}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Molecule Man", Count: 4}, {Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 14}, {Name: "Island", Count: 4},
	},
}

var offZoneMiracleDecksB = [2][]cardset.Entry{
	{
		{Name: "Aminatou, Veil Piercer", Count: 6}, {Name: "Glorious Anthem", Count: 6}, {Name: "Pacifism", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Lorehold, the Historian", Count: 4}, {Name: "Lightning Bolt", Count: 6}, {Name: "Glorious Anthem", Count: 4},
		{Name: "Raging Goblin", Count: 6}, {Name: "Mountain", Count: 12}, {Name: "Plains", Count: 8},
	},
}

func testOffZoneMiracle(t *testing.T, decks [2][]cardset.Entry) {
	t.Helper()
	_, set, gd := lockstepPool(t, offZoneUnearthPool, decks)
	if !SetDeckSupported(set, gd[0]) || !SetDeckSupported(set, gd[1]) {
		t.Fatal("the miracle decks are not playable in the offzonegrants pool")
	}
	before := dirac.MiracleGrantStats.Reveals.Load()
	lockstepRunPolicy(t, offZoneUnearthPool, decks, 64, true, nil, nil, castFirstPolicy)
	if n := dirac.MiracleGrantStats.Reveals.Load() - before; n == 0 {
		t.Fatal("no granted miracle reveal was queued; the lockstep proves nothing")
	} else {
		t.Logf("granted miracle reveals: %d", n)
	}
}

func TestOffZoneMiracleLockstep(t *testing.T) {
	testOffZoneMiracle(t, offZoneMiracleDecks)
}

func TestOffZoneMiracleLockstepEnchantments(t *testing.T) {
	testOffZoneMiracle(t, offZoneMiracleDecksB)
}
