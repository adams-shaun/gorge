package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The TargetValidTargeting$ lockstep for the counters the corpus classes
// beside Fugitive Droid and Turn Aside: Confound and Intervene
// ("Creature.inRealZoneBattlefield", a dot before the zone word), Avoid Fate
// (a spell-side ValidTgts$ Instant,Aura filter beside the clause), Dawn
// Charm's counter mode (TargetValidTargeting$ You, a player target) and
// Vigilant Martyr (an ability counter with the same dot form). The opponent
// aims Shock, Lightning Bolt and Pacifism at the first deck's creatures and
// face and Giant Growth at its own.
var kctrvtgtDecks = [2][]cardset.Entry{
	{
		{Name: "Confound", Count: 3}, {Name: "Intervene", Count: 3}, {Name: "Avoid Fate", Count: 3},
		{Name: "Dawn Charm", Count: 3}, {Name: "Vigilant Martyr", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Giant Growth", Count: 2}, {Name: "Island", Count: 8}, {Name: "Plains", Count: 6},
		{Name: "Forest", Count: 6},
	},
	{
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 3}, {Name: "Pacifism", Count: 3},
		{Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 5},
		{Name: "Plains", Count: 4},
	},
}

func TestCtrvtgtLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ctrvtgt", kctrvtgtDecks, 400, "KCTRVTGT_SEEDS")
}

// TestCtrvtgtLockstepMutation: with TgtSpellTgting ignoring the targeting
// clause (and the spell-side filter), some seed diverges, so the pool does
// meet an untargeting or wrong-targeting spell the cards must refuse.
func TestCtrvtgtLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugCtrShapesOff = true
	defer func() { dirac.DebugCtrShapesOff = false }()
	if !lockstepDiverges(t, "ctrvtgt", kctrvtgtDecks, 400) {
		t.Fatal("no seed diverged with the clause ignored: no spell met Confound, Intervene, Avoid Fate or Dawn Charm that its clause excludes")
	}
}
