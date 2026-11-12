package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stackManaAbility reports whether an AB$ Mana is no mana ability by CR
// 605.1a because its chain draws or mills (cards.IsManaAbilitySA's library
// gate: Chromatic Sphere, the five Eggs). gorge resolves it on the stack: the
// ability is offered at priority, pays its cost as an activated ability's
// (a mana ability could be activated mid-payment; this one cannot), and on
// resolution adds its mana and runs the rest of the chain. finishMana lowers
// it as a loyalty ability's mana: an activated ability whose program is the
// mana, then the chain.
//
// A cost that moves a library card (Mill<N>) or a target anywhere in the chain
// is left to the existing refusals.
func (lc *lowerCtx) stackManaAbility(sa *cards.SA, ab *lAbility) bool {
	if sa == nil || ab.Loyalty || ab.Kind != akMana || sa.Kind != "AB" || sa.API != "Mana" || cards.IsManaAbilitySA(sa) {
		return false
	}
	if dbManaCostMovesLibrary(sa.Params["Cost"]) {
		return false
	}
	for s, d := sa, 0; s != nil && d < 16; s, d = s.Sub, d+1 {
		if strings.TrimSpace(s.Params["ValidTgts"]) != "" || strings.TrimSpace(s.Params["TargetType"]) != "" {
			return false
		}
	}
	return true
}
