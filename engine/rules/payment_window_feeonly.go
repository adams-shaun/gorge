package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// feeOnlyConversionStalls reports whether activating ma now cannot bring the
// pending payment closer to paid. ma is a repeatable fee-only mana ability
// (Bog Initiate's "{1}: Add {B}.") that makes no more mana than its fee
// spends, so it only converts pool mana into a colour of its own: it helps
// exactly when that colour fills a fixed pip the pool still lacks. A
// generic-only remainder, or a colour the pool already covers, is not helped,
// and offering it makes the window a no-progress cycle (the fee eats the mana
// the activation just made). A flexible pip, an any-type rider or an
// unpriceable fee fails open: false. dirac's windowFeeStalls
// (mana_filterrepeat.go) is the same rule.
func (e *Engine) feeOnlyConversionStalls(pc *pendingCast, p state.PlayerID, id state.ObjID, ma *cards.SA) bool {
	if pc.mayPlayIgnore || pc.mayPlayIgnoreType || !repeatableFeeOnlyMana(e, p, id, ma) {
		return false
	}
	fee := e.manaAbilityCC(p, id, ma).Cost
	var made state.Mana
	e.addPotentialManaOf(&made, ma)
	amount, colours := int32(0), 0
	for _, n := range made {
		if n > 0 {
			colours++
			amount = n
		}
	}
	if colours > 1 {
		amount = 1 // a choice among colours: one mana of the chosen one
	}
	if amount > fee.Generic+fee.Colored.Total() {
		return false
	}
	comp := e.manaToPay(pc)
	if len(comp.Hybrid) != 0 || len(comp.Phyrexian) != 0 || len(comp.Twobrid) != 0 || len(comp.HybridPhyrexian) != 0 {
		return false
	}
	pool := pay.AvailableFor(asPayer(e), p, paymentForCast(pc, comp)).Pool
	short := func(pool state.Mana) (n int32) {
		for i := range pool {
			if comp.Colored[i] > pool[i] {
				n += comp.Colored[i] - pool[i]
			}
		}
		return n
	}
	before := short(pool)
	// The fee's coloured pips are paid first, then its generic part from the
	// mana no fixed pip of the payment needs, then from any (pay/solve.go).
	for i := range pool {
		if fee.Colored[i] > pool[i] {
			return false // the live gate already withholds an unpayable fee
		}
		pool[i] -= fee.Colored[i]
	}
	for g := fee.Generic; g > 0; g-- {
		spent := false
		for i := range pool {
			if pool[i] > comp.Colored[i] {
				pool[i]--
				spent = true
				break
			}
		}
		if spent {
			continue
		}
		for i := range pool {
			if pool[i] > 0 {
				pool[i]--
				spent = true
				break
			}
		}
		if !spent {
			return false
		}
	}
	for c, n := range made {
		if n == 0 {
			continue
		}
		after := pool
		after[c] += amount
		if short(after) < before {
			return false
		}
	}
	return true
}
