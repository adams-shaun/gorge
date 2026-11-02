package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Phyrexian mana lockstep (CR 107.4f): Gut Shot,
// Mutagenic Growth, Mental Misstep, Dismember and Vault Skirge, each Phyrexian pip announced as its colour or two life. One deck
// holds the pips' colours, the other few of them, so both faces are
// offered and the life face is often the only one; burn brings life low
// enough that the two-life face stops being payable. The expected answer at
// every decision is gorge's. Gitaxian Probe lowers but stays out: gorge
// paces its private look with a one-option "Continue" ask (effects
// poseLookAck) the kernel does not pose, a divergence whenever single-option
// decisions are not elided.
var kphyDecks = [2][]cardset.Entry{
	{
		{Name: "Gut Shot", Count: 4}, {Name: "Mutagenic Growth", Count: 4}, {Name: "Lightning Bolt", Count: 3},
		{Name: "Dismember", Count: 3}, {Name: "Vault Skirge", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Lightning Bolt", Count: 3}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 6}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Mental Misstep", Count: 3}, {Name: "Gut Shot", Count: 3}, {Name: "Dismember", Count: 2},
		{Name: "Savannah Lions", Count: 5}, {Name: "Wind Drake", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Walking Corpse", Count: 3}, {Name: "Opt", Count: 2}, {Name: "Plains", Count: 8}, {Name: "Island", Count: 7},
	},
}

func TestPhyLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "phy", kphyDecks, 1000, "KPHY_SEEDS")
}

// TestPhyLockstepMutation proves the pool pays Phyrexian pips with life:
// without the payment, some seed diverges.
func TestPhyLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPhyrexianLife = true
	defer func() { dirac.DebugNoPhyrexianLife = false }()
	if !lockstepDiverges(t, "phy", kphyDecks, 40) {
		t.Fatal("no seed diverged without the two-life payment: the pool never pays a Phyrexian pip with life")
	}
}
