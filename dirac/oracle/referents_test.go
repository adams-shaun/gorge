package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The filter-referent lockstep: Chorale of the Void
// puts a creature from the DEFENDING player's graveyard onto the battlefield
// (OwnedBy TriggeredDefendingPlayer); Cruel Calculations draws for the cards
// milled into the targeted player's graveyard this turn (OwnedBy Targeted);
// Saw sacrifices a permanent other than the attacker (NotDefinedTriggeredAttacker);
// Light of Judgment destroys an Equipment attached to its target
// (AttachedTo Targeted, an optional ChooseCard). Thought Scour mills.
var kreferentsDecks = [2][]cardset.Entry{
	{
		{Name: "Chorale of the Void", Count: 4}, {Name: "Saw", Count: 4}, {Name: "Light of Judgment", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Raging Goblin", Count: 4}, {Name: "Bonesplitter", Count: 3},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Cruel Calculations", Count: 4}, {Name: "Thought Scour", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Bonesplitter", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 8},
	},
}

func TestReferentsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "referents", kreferentsDecks, 1000, "KREFERENTS_SEEDS")
}

// TestReferentsLockstepMutation proves the off-battlefield owner role reads
// the right seat: swapped, Chorale offers its own controller's graveyard or
// Cruel Calculations counts the wrong player's graveyard.
func TestReferentsLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugReferentOwnerSwap = true
	defer func() { dirac.DebugReferentOwnerSwap = false }()
	if !lockstepDiverges(t, "referents", kreferentsDecks, 1000) {
		t.Fatal("no seed diverged with the owner role swapped: the pool does not exercise it")
	}
}
