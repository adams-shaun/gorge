package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// repeatableFeeOnlyMana reports a mana ability whose whole cost is mana (Bog
// Initiate's "{1}: Add {B}.", Orochi Leafcaller's "{G}: ...", Agent of
// Stromgald's "{R}: Add {B}."): no {T}, no sacrifice and no activation limit,
// so a payment activates it as often as it can fund the fee (CR 605.3a: mana
// abilities are activated while paying, and nothing here is used up).
func (e *Engine) repeatableFeeOnlyMana(p state.PlayerID, id state.ObjID, ma *cards.SA) bool {
	cost := e.manaAbilityCC(p, id, ma).Cost
	if cost.Generic+cost.Colored.Total() <= 0 {
		return false
	}
	want := costvocab.Cost{Generic: cost.Generic, Colored: cost.Colored}
	if !cost.Equal(&want) {
		return false
	}
	return strings.TrimSpace(ma.ParamStr(cards.PKActivationLimit)) == "" &&
		strings.TrimSpace(ma.ParamStr(cards.PKGameActivationLimit)) == ""
}

// addPotentialRepeatable lifts to potentialUnbounded each colour a
// repeatable fee-only ability makes: the bound counts a source's production
// once, but the fee converts mana into the ability's colour once per
// activation, so Bog Initiate beside two other lands funds {B}{B}.
func (e *Engine) addPotentialRepeatable(out *state.Mana, p state.PlayerID, id state.ObjID, ma *cards.SA) {
	if !e.repeatableFeeOnlyMana(p, id, ma) {
		return
	}
	var made state.Mana
	e.addPotentialManaOf(&made, ma)
	for c, n := range made {
		if n > 0 {
			out[c] = potentialUnbounded
		}
	}
}
