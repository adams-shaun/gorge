package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// windowOnlyStallingConverters reports whether every mana ability source id
// offers a resolution-time payment window (a triggered cost's, cumulative
// upkeep's, echo's) is a repeatable fee-only conversion that cannot fill what
// amount still owes (feeOnlyConversionStalls). The window carries no pending
// cast, so paymentSourceUseful's clamp never prices it, and a seat that always
// takes the first listed source re-activates Bog Initiate's "{1}: Add {B}."
// forever: each fee eats the mana the last activation made (tail-49 pair 47,
// Lapis Lazuli Talisman's "you may pay {3}").
func (e *Engine) windowOnlyStallingConverters(p state.PlayerID, id state.ObjID, amount Cost, d paymentDescriptor) bool {
	n := 0
	var pool state.Mana
	for _, ma := range e.availableManaAbilities(p, id) {
		if pay.InstantSpeedOnly(ma) {
			continue
		}
		if !repeatableFeeOnlyMana(e, p, id, ma) {
			return false
		}
		if n == 0 {
			pool = pay.AvailableFor(asPayer(e), p, d).Pool
		}
		n++
		if !feeOnlyConversionStalls(e, p, id, ma, amount, pool) {
			return false
		}
	}
	return n > 0
}

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
//
// comp is the composition still owed and pool the mana the payment may spend;
// the caller (paymentSourceUseful) prices both and rules out an any-type rider.
func feeOnlyConversionStalls(pp potentialPricer, p state.PlayerID, id state.ObjID, ma *cards.SA, comp Cost, pool state.Mana) bool {
	if !repeatableFeeOnlyMana(pp, p, id, ma) {
		return false
	}
	fee := pp.manaAbilityCC(p, id, ma).Cost
	var made state.Mana
	pp.addPotentialManaOf(&made, ma)
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
	if len(comp.Hybrid) != 0 || len(comp.Phyrexian) != 0 || len(comp.Twobrid) != 0 || len(comp.HybridPhyrexian) != 0 {
		return false
	}
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
	// pool in the order pay/solve.go spends it: {C}, then W, U, B, R, G. That
	// order ignores the pips still owed, so it can spend the very colour the
	// activation is meant to supply.
	for i := range pool {
		if fee.Colored[i] > pool[i] {
			return false // the live gate already withholds an unpayable fee
		}
		pool[i] -= fee.Colored[i]
	}
	for g := fee.Generic; g > 0; g-- {
		spent := false
		for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
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
