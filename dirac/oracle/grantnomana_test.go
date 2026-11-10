package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A granted mana ability (Cryptolith Rite) beside a source of no colour
// (Command Tower, Path of Ancestry: commander identity, nothing without a
// commander, CR 903.4f): gorge's prefix search taps the no-colour gap in
// battlefield order with the granted ones, so the kernel's must too.
var kgrantNoManaDecks = [2][]cardset.Entry{
	{{Name: "Cryptolith Rite", Count: 8}, {Name: "Path of Ancestry", Count: 6}, {Name: "Command Tower", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Craw Wurm", Count: 6}, {Name: "Forest", Count: 8}},
	{{Name: "Path of Ancestry", Count: 8}, {Name: "Savannah Lions", Count: 12}, {Name: "Shock", Count: 6},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 6}},
}

// the same decks with every no-colour source a Forest (deck 1 keeps its own
// Path of Ancestry: it has no grant, so its search never starts)
var kgrantNoManaControl = [2][]cardset.Entry{
	{{Name: "Cryptolith Rite", Count: 8}, {Name: "Forest", Count: 18},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Craw Wurm", Count: 6}},
	kgrantNoManaDecks[1],
}

func TestGrantNoManaGapLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "grantnomana", kgrantNoManaDecks, 100, "KGRANTNOMANA_SEEDS")
	// the control: without a no-colour source the same board plays clean,
	// so a divergence above is the gap, not the grant
	if lockstepDiverges(t, "grantnomana", kgrantNoManaControl, 100) {
		t.Error("the control decks (Forest for every Path/Command Tower) diverge: the grant alone breaks the lockstep")
	}
}
