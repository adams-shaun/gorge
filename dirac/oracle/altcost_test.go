package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The mana alternative cost lockstep: Blasphemous Edict's "you
// may pay {B} rather than pay this spell's mana cost if there are thirteen
// or more creatures on the battlefield" (gorge alternativeCosts: an
// AltCostIndex 1 cast at the alternative price), between two creature
// decks that fill the battlefield. The oracle plays gorge's payment-planner
// surface, which skips AltCostIndex options (payment_plan.go), so the
// alternative-cost cast is offered only when the floating pool pays it: with
// the thirteen creatures out and an untapped Swamp it is not (19 of these
// 1000 seeds; an affordability-only offer diverged on every one). No pool
// card floats {B} at sorcery speed, so the offered cast itself is not
// reached. The expected answer at every decision is gorge's.
var kaltcostDecks = [2][]cardset.Entry{
	{
		{Name: "Blasphemous Edict", Count: 6}, {Name: "Raise the Alarm", Count: 8}, {Name: "Savannah Lions", Count: 10},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Llanowar Elves", Count: 8}, {Name: "Grizzly Bears", Count: 12}, {Name: "Forest", Count: 20},
	},
}

func TestAltcostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "altcost", kaltcostDecks, 1000, "KALTCOST_SEEDS")
}
