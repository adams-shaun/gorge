package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A smoke test for the oppentered fixture table, NOT evidence for the
// per-opponent read. Whiplash Trap's "if an opponent had two or more creatures
// enter the battlefield under their control this turn, you may pay {U} rather
// than pay this spell's mana cost" is offered only while a floating pool pays
// the {U} (payment-planner surface, cast_altfree.go offerAltFreeHand, as
// Blasphemous Edict in altcost_test.go), and no pool card floats {U}: the gate
// is never read in these games, so this test passes with the kernel arm
// reverted. It proves the lowered table plays clean (the entered ledger, the
// Trap's targets and its full-cost cast). The head's gorge-vs-kernel evidence
// is the atom count:PlayerCountOpponents$ConditionGE<n> ThisTurnEntered_
// (atomdiff_count_oppentered.go) on random states, and dirac.TestOppMemberExprFigure,
// which fails on a kernel that reads the opponent's creatures from the caster's side.
var koppenteredDecks = [2][]cardset.Entry{{
	{Name: "Whiplash Trap", Count: 8}, {Name: "Elite Vanguard", Count: 6}, {Name: "Savannah Lions", Count: 6},
	{Name: "Island", Count: 10}, {Name: "Plains", Count: 10},
}, {
	{Name: "Whiplash Trap", Count: 6}, {Name: "Elite Vanguard", Count: 8}, {Name: "Savannah Lions", Count: 8},
	{Name: "Island", Count: 9}, {Name: "Plains", Count: 9},
}}

func TestOppEnteredLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "oppentered", koppenteredDecks, 300, "KOPPENTERED_SEEDS")
}
