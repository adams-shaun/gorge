package rules

import (
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// RollModifyCostPayable is the effects Host hook: pay.RollModifyCostPayable.
func (e *Engine) RollModifyCostPayable(p state.PlayerID, cost string) bool {
	return pay.RollModifyCostPayable(asPayer(e), p, cost)
}

// PayRollModifyCost is the effects Host hook: pay.PayRollModifyCost.
func (e *Engine) PayRollModifyCost(p state.PlayerID, cost string) bool {
	return pay.PayRollModifyCost(asPayer(e), p, cost)
}
