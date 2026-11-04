package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Damage dealt to a planeswalker that is also a creature has both results
// (CR 120.3c, 120.3e): it loses that many loyalty counters and the damage is
// marked. Kaito, Bane of Nightmares is a creature on his controller's turn,
// and Unable to Scream makes an enchanted planeswalker a creature on every
// turn, so a 1/1 that hits him leaves a planeswalker with one loyalty fewer
// and a 0/2 creature with one damage marked.
func TestDamageToCreatureWalkerRemovesLoyaltyAndMarks(t *testing.T) {
	if isolate(t) {
		return
	}
	walkers := []cardset.Entry{
		{Name: "Kaito, Bane of Nightmares", Count: 12}, {Name: "Unable to Scream", Count: 6}, {Name: "Wind Drake", Count: 4},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 9},
	}
	burn := []cardset.Entry{
		{Name: "Shock", Count: 8}, {Name: "Hill Giant", Count: 8}, {Name: "Raging Goblin", Count: 8},
		{Name: "Mountain", Count: 16},
	}
	lockstepRun(t, "walkerdamage", [2][]cardset.Entry{walkers, burn}, 40, true, nil)
}
