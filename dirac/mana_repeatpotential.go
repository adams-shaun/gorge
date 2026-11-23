package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A repeatable fee-only mana ability ("{R}: Add {B}", Agent of Stromgald; see
// kop.FeeOnlyManaAb) in a trigger's own cost gate. gorge's potentialManaFor
// (rules potential.go) admits it once its fee is covered and then lifts each
// colour it makes to potentialUnbounded (potential_repeat_fee.go
// addPotentialRepeatable): a payment may activate it as often as it can fund
// the fee, so the bound reads the colour as many. The trigger pay gate
// (trigCostPotentialPays, the subBound descriptor) therefore offers "pay" for a
// cumulative upkeep of {4} beside a Plains, a Mountain and an Agent of
// Stromgald, whose real yield is two mana: the payer elects pay, the charge
// fails (CR 118.3) and the payer declines. The cast bound (castBoundDrops,
// repeatBound) reads it the same way: gorge lifts it for every payment's bound.
// Every other payment keeps the source's production counted once.

// addOrMergeRepeat is potCensus.addOrMerge for the fixed production of ab, plus
// the unbounded lift of its colours when ab is a repeatable fee-only ability
// and d is the trigger pay gate's descriptor or the cast bound's (repeatBound).
// The census (cen.on) keeps the natural amount.
func (s *State) addOrMergeRepeat(c *potCensus, sl Slot, ab *kop.Ability, d *payDesc, drop uint8, add, fold func(Slot, uint8, uint8)) {
	c.addOrMerge(sl, ab, drop, add, fold)
	if c.on || d == nil || !(d.subBound || d.repeatBound) || !kop.FeeOnlyManaAb(ab) {
		return
	}
	fold(sl, uint8(ab.ManaColors), potentialOpenUnits)
}
