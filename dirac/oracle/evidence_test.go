package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The collect-evidence ward lockstep: Axebane Ferox's
// Ward—Collect evidence 4. The opponent targets it with burn and Murder
// while its graveyard fills with spent spells and dead creatures, so the
// ward is paid (a selection of 1..all graveyard cards, short of four mana
// value countering anyway), declined, or unpayable. The expected answer at
// every decision is gorge's.
var kevidenceDecks = [2][]cardset.Entry{
	{
		{Name: "Axebane Ferox", Count: 10}, {Name: "Grizzly Bears", Count: 6}, {Name: "Giant Growth", Count: 3},
		{Name: "Forest", Count: 21},
	},
	{
		{Name: "Murder", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 10},
	},
}

func TestEvidenceLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "evidence", kevidenceDecks, 1000, "KEVIDENCE_SEEDS")
}

// TestEvidenceLockstepMutation proves the pool pays the ward: with a paid
// selection countering anyway, some seed diverges.
func TestEvidenceLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoEvidence = true
	defer func() { dirac.DebugNoEvidence = false }()
	if !lockstepDiverges(t, "evidence", kevidenceDecks, 60) {
		t.Fatal("no seed diverged with a paid evidence selection countering: the pool never pays the ward")
	}
}
