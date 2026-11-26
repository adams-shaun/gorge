package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted six-zone madness lockstep (CR 702.35a-b): Falkenrath Gorger's
// "each Vampire creature card you own that isn't on the battlefield has
// madness, the madness cost equal to its mana cost" gives the kernel the same
// discard into exile, the same cast-or-graveyard ask and the same cast at the
// card's own mana cost that gorge's derived-keyword read gives (rules
// altcast.go madnessReplacementApplies, offerMadness, madnessCostLabel).
//
// The pool is the off-battlefield statics fixture (offzonegrants). Deck 1
// runs the grantor, a Vampire without printed madness and a looting spell
// that discards it; deck 2 only plays creatures and lands.
var offZoneMadnessDecks = [2][]cardset.Entry{
	{
		{Name: "Falkenrath Gorger", Count: 8}, {Name: "Vampire Interloper", Count: 8}, {Name: "Faithless Looting", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 8}, {Name: "Raging Goblin", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Island", Count: 8},
	},
}

func TestOffZoneMadnessLockstep(t *testing.T) {
	_, set, gd := lockstepPool(t, offZoneUnearthPool, offZoneMadnessDecks)
	if !SetDeckSupported(set, gd[0]) || !SetDeckSupported(set, gd[1]) {
		t.Fatal("the madness decks are not playable in the offzonegrants pool")
	}
	before := dirac.MadnessGrantStats.Exiled.Load()
	lockstepRunPolicy(t, offZoneUnearthPool, offZoneMadnessDecks, 64, true, nil, nil, castFirstPolicy)
	if n := dirac.MadnessGrantStats.Exiled.Load() - before; n == 0 {
		t.Fatal("no granted-madness discard was exiled; the lockstep proves nothing")
	} else {
		t.Logf("granted madness discards exiled: %d", n)
	}
}
