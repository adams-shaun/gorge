package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Temple of the False God lockstep: its only mana ability, {T}: Add
// {C}{C}, can be activated only while its controller controls five or more
// lands (IsPresent$ with PresentCompare$, CR 602.5b; gorge
// pay.ManaActivationGateHolds). Below five lands the Temple is no source at
// all; at five it pays two generic. Craw Wurm's {4}{G}{G} and Hill Giant's
// {3}{R} lean on it. The expected answer at every decision is gorge's.
var ktemplefgDecks = [2][]cardset.Entry{
	{
		{Name: "Temple of the False God", Count: 10}, {Name: "Forest", Count: 12}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Craw Wurm", Count: 8}, {Name: "Shock", Count: 4},
	},
	{
		{Name: "Temple of the False God", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 4},
		{Name: "Hill Giant", Count: 8}, {Name: "Craw Wurm", Count: 4}, {Name: "Shock", Count: 6},
	},
}

func TestTempleFalseGodLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "templefg", ktemplefgDecks, 400, "KTEMPLEFG_SEEDS")
}

// TestTempleFalseGodLockstepMutation: with the gate read always on, some
// seed diverges.
func TestTempleFalseGodLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.SetDebugEntersTappedMutate(5)
	defer dirac.SetDebugEntersTappedMutate(0)
	if n := lockstepRun(t, "templefg", ktemplefgDecks, 100, false, nil); n == 0 {
		t.Fatal("no seed diverged with the gate always on")
	}
}
