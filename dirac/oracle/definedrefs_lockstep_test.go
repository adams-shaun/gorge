package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Defined$ referents lockstep: Regenerate over Defined$ Valid
// Creature.YouCtrl (Wrap in Vigor, Loxodon Hierarch, Wail of the Nim),
// Amulet of Safekeeping's counter unless the targeting spell's controller
// pays (UnlessPayer$ TriggeredSourceSAController), and the each-player hand
// moves of Mind Swords and Sadistic Augermage (DefinedPlayer$ Player).
var kdefinedRefsDecks = [2][]cardset.Entry{
	{{Name: "Loxodon Hierarch", Count: 4}, {Name: "Wrap in Vigor", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Amulet of Safekeeping", Count: 4}, {Name: "Forest", Count: 12}, {Name: "Plains", Count: 12}},
	{{Name: "Sadistic Augermage", Count: 4}, {Name: "Mind Swords", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Wail of the Nim", Count: 4}, {Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 12}},
}

func TestDefinedRefsLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "definedrefs", kdefinedRefsDecks, 200, "KDEFINEDREFS_SEEDS")
}
