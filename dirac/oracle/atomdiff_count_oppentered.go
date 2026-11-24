package oracle

import "github.com/adams-shaun/gorge/dirac"

// The opponents' entered-creature condition (gorge playerCountCondition over
// opponentGroup, Whiplash Trap's "an opponent had two or more creatures enter
// the battlefield under their control this turn"): the number of opponents
// whose own ThisTurnEntered_ count reaches n, each member's YouCtrl bound to
// that member. It is ExOppExtreme's OppExtCountCmp with OppExtMemberExpr, so
// a pod's opponents are walked one by one. The entered ledger is kept only
// for a table that reads it, hence Needs.
func init() {
	for _, h := range []string{
		"PlayerCountOpponents$ConditionGE1 ThisTurnEntered_Battlefield_Creature.YouCtrl",
		"PlayerCountOpponents$ConditionGE2 ThisTurnEntered_Battlefield_Creature.YouCtrl",
	} {
		RegisterCountAtom(CountAtom{Name: "count:" + h, GorgeHead: h, Needs: dirac.BankEntered})
	}
}
