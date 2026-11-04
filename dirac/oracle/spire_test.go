package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Spire of Industry lockstep: {T}: Add {C}, and {T}, Pay 1 life: Add
// one mana of any colour, activated only while its controller controls an
// artifact (IsPresent$, CR 602.5b; gorge pay.ManaActivationGateHolds). With
// no artifact the Spire is a colourless land; with an Ornithopter out it is
// Starting Town's last-resort life alternative beside {C}. Both decks lean
// on it for their colours. The expected answer at every decision is gorge's.
var kspireDecks = [2][]cardset.Entry{
	{
		{Name: "Spire of Industry", Count: 14}, {Name: "Ornithopter", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Savannah Lions", Count: 4}, {Name: "Shock", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Spire of Industry", Count: 12}, {Name: "Ornithopter", Count: 4}, {Name: "Wind Drake", Count: 6},
		{Name: "Shock", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 6},
	},
}

func TestSpireLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "spire", kspireDecks, 600, "KSPIRE_SEEDS")
}

// TestSpireLockstepMutation: with the gate read always on, and with the
// paid life skipped, some seed diverges.
func TestSpireLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.SetDebugEntersTappedMutate(5)
	n := lockstepRun(t, "spire", kspireDecks, 200, false, nil)
	dirac.SetDebugEntersTappedMutate(0)
	if n == 0 {
		t.Error("no seed diverged with the gate always on")
	}
	dirac.DebugNoPainDamage = true
	n = lockstepRun(t, "spire", kspireDecks, 200, false, nil)
	dirac.DebugNoPainDamage = false
	if n == 0 {
		t.Error("no seed diverged with the paid life skipped")
	}
}
