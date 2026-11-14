package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The attack-target lockstep: Sealock Monster's "can't attack unless
// defending player controls an Island" (CantAttack UnlessDefender$
// controlsIsland) offers the same attack options on both sides. One deck holds
// the Monsters and the Islands that cast them; the other holds Islands in the
// second pool, none in the first.
var attackTargetDecks = [2][]cardset.Entry{
	{
		{Name: "Sealock Monster", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 16}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
		{Name: "Forest", Count: 14}, {Name: "Mountain", Count: 14},
	},
}

// attackTargetEncoreDeck holds the Encore creatures, a planeswalker for the
// token to choose over its opponent, and burn to put the creatures in the
// graveyard; both seats play it.
var attackTargetEncoreDeck = []cardset.Entry{
	{Name: "Impulsive Pilferer", Count: 4}, {Name: "Coastline Marauders", Count: 4},
	{Name: "Kitesail Skirmisher", Count: 4}, {Name: "Chandra, Novice Pyromancer", Count: 4},
	{Name: "Shock", Count: 6}, {Name: "Hill Giant", Count: 2},
	{Name: "Mountain", Count: 8}, {Name: "Island", Count: 8},
}

func TestAttackTargetsEncoreLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "attacktargets", [2][]cardset.Entry{attackTargetEncoreDeck, attackTargetEncoreDeck}, 240, "KATTACKTARGETS_SEEDS")
}

func TestAttackTargetsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "attacktargets", attackTargetDecks, 240, "KATTACKTARGETS_SEEDS")
}
