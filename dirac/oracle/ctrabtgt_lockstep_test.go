package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The TargetValidTargeting$ lockstep for a counter that also takes an
// activated or triggered ability: Diplomatic Escort ("Counter target spell or
// ability that targets a creature") against the opponent's Prodigal
// Pyromancer activations (aimed at a creature or a player), Man-o'-War's enters
// trigger (aimed at a creature), Elvish Visionary's and Divination's untargeted
// draws, and burn. The first deck carries creatures (and its own Pyromancer
// and Man-o'-War) so abilities and spells have creatures to aim at on both
// sides.
var kctrabtgtDecks = [2][]cardset.Entry{
	{
		{Name: "Diplomatic Escort", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Prodigal Pyromancer", Count: 2},
		{Name: "Man-o'-War", Count: 2}, {Name: "Island", Count: 12}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4},
	},
	kctrshapesDeckB,
}

func TestCtrabtgtLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ctrabtgt", kctrabtgtDecks, 400, "KCTRABTGT_SEEDS")
}

// TestCtrabtgtLockstepMutation: with TgtStackTgting admitting any ability,
// some seed diverges, so the pool does meet an activated or triggered ability
// that does not target a creature (Prodigal Pyromancer at a player, an
// untargeted trigger) that Diplomatic Escort must not be offered.
func TestCtrabtgtLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrAbTgtOff = true
	defer func() { dirac.DebugCtrAbTgtOff = false }()
	if !lockstepDiverges(t, "ctrabtgt", kctrabtgtDecks, 400) {
		t.Fatal("no seed diverged with every ability admitted: no ability that does not target a creature met Diplomatic Escort")
	}
}
