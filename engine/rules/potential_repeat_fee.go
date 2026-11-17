package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// potentialPricer is the slice of *Engine the potential walk's per-ability
// reads need: the priced cost of a mana ability and its folded production.
type potentialPricer interface {
	manaAbilityCC(p state.PlayerID, source state.ObjID, ma *cards.SA) *compiledCost
	addPotentialManaOf(m *state.Mana, ma *cards.SA)
}

// repeatableFeeOnlyMana reports a mana ability whose whole cost is mana (Bog
// Initiate's "{1}: Add {B}.", Orochi Leafcaller's "{G}: ...", Agent of
// Stromgald's "{R}: Add {B}."): no {T}, no sacrifice and no activation limit,
// so a payment activates it as often as it can fund the fee (CR 605.3a: mana
// abilities are activated while paying, and nothing here is used up).
func repeatableFeeOnlyMana(pp potentialPricer, p state.PlayerID, id state.ObjID, ma *cards.SA) bool {
	cost := pp.manaAbilityCC(p, id, ma).Cost
	if cost.Generic+cost.Colored.Total() <= 0 {
		return false
	}
	want := costvocab.Cost{Generic: cost.Generic, Colored: cost.Colored}
	if !cost.Equal(&want) {
		return false
	}
	_, limited := ma.Param(cards.PKActivationLimit)
	_, gameLimited := ma.Param(cards.PKGameActivationLimit)
	return !limited && !gameLimited
}

// addPotentialRepeatable lifts to potentialUnbounded each colour a
// repeatable fee-only ability makes: the bound counts a source's production
// once, but the fee converts mana into the ability's colour once per
// activation, so Bog Initiate beside two other lands funds {B}{B}.
func addPotentialRepeatable(pp potentialPricer, out *state.Mana, p state.PlayerID, id state.ObjID, ma *cards.SA) {
	if !repeatableFeeOnlyMana(pp, p, id, ma) {
		return
	}
	var made state.Mana
	pp.addPotentialManaOf(&made, ma)
	for c, n := range made {
		if n > 0 {
			out[c] = potentialUnbounded
		}
	}
}
