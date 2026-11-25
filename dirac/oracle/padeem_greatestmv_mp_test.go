package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Padeem, Consul of Innovation: "At the beginning of your upkeep, if you
// control the artifact with the highest mana value or tied for the highest
// mana value, draw a card." (Count$Valid Artifact.greatestCMC_Artifact+YouCtrl;
// gorge greatestCMC_<prop>, kernel kop.W7GreatestMVArtifact). Every seat holds
// artifacts of several mana values, so the condition turns on the whole
// battlefield. The mutation (DebugWord7Flip) inverts the word.
var padeemDeck = []cardset.Entry{
	{Name: "Padeem, Consul of Innovation", Count: 6}, {Name: "Sol Ring", Count: 3}, {Name: "Mind Stone", Count: 3},
	{Name: "Juggernaut", Count: 3}, {Name: "Millstone", Count: 3},
	{Name: "Island", Count: 22},
}

// TestFourSeatPadeemGreatestMVLockstep: four-seat Padeem games play in
// lockstep, and the inverted word diverges.
func TestFourSeatPadeemGreatestMVLockstep(t *testing.T) {
	atkShapeLockstep(t, padeemDeck, 16, 0, 0, nil, nil, &dirac.DebugWord7Flip)
}
