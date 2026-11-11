package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Ivorytusk Fortress: "untap each creature you control with a +1/+1 counter
// on it during each other player's untap step" (gorge finishUntapStep's
// foreign scan over untapOtherStaticsMatch). Battlegrowth puts the counters
// on; the attacking and mana-tapping creatures it lands on untap again on
// the opponent's turn. The expected answer at every decision is gorge's.
var kivorytuskDecks = [2][]cardset.Entry{
	{
		{Name: "Ivorytusk Fortress", Count: 5}, {Name: "Battlegrowth", Count: 7},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Shock", Count: 6},
		{Name: "Mountain", Count: 11}, {Name: "Forest", Count: 11},
	},
}

func TestIvorytuskFortressLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "ivorytusk", kivorytuskDecks, 200, "KIVORYTUSK_SEEDS")
}

// TestIvorytuskFortressMutation: with the kernel's foreign untap scan off,
// the pool diverges from gorge, so the lockstep above exercises it.
func TestIvorytuskFortressMutation(t *testing.T) {
	old := dirac.DebugUntapOtherOff
	dirac.DebugUntapOtherOff = true
	defer func() { dirac.DebugUntapOtherOff = old }()
	if !lockstepDiverges(t, "ivorytusk", kivorytuskDecks, 200) {
		t.Fatal("the pool never reaches an Ivorytusk Fortress foreign untap")
	}
}
