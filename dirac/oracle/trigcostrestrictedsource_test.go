package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A trigger's own mana cost is offered when the payer's potential mana can
// reach it (gorge resPayNeedsMana), and its untapped-source gate counts any
// available mana ability whatever its RestrictValid$: the restriction is
// priced against the payment afterwards. Ant-Man, Colony Commander's "you may
// pay {1}" on attack, with every land tapped and a Hydraulic Helper (its {U}
// can't pay a nonartifact spell, so it pays this) untapped, offers pay.
var ktrigcostrestrictedDecks = [2][]cardset.Entry{{
	{Name: "Ant-Man, Colony Commander", Count: 6}, {Name: "Hydraulic Helper", Count: 10},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
	{Name: "Forest", Count: 8}, {Name: "Island", Count: 9},
}, {
	{Name: "Ant-Man, Colony Commander", Count: 6}, {Name: "Hydraulic Helper", Count: 10},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
	{Name: "Forest", Count: 8}, {Name: "Island", Count: 9},
}}

func TestTrigCostRestrictedSourceLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "trigcostrestricted", ktrigcostrestrictedDecks, 200, "KTRIGCOSTRESTRICTED_SEEDS")
}
