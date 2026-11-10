package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lower_effectdurations.go: the two Effect shapes that outlive the turn and
// have no emblem of their own.
//
//   - A may-play grant over the chosen card that lasts to the end of the
//     controller's next turn (a Dig, ChooseCard, then an Effect with no
//     RememberObjects$): the per-card may-play record carries that lifetime
//     (EndOfYourNextTurn), the same instruction a RememberObjects$ ChosenCard
//     spelling lowers to.
//   - "It loses all abilities for as long as CARDNAME remains on the
//     battlefield" over the one targeted permanent: a host-bound
//     LoseAbilities override on it, which ends when the host leaves.

// chosenMayPlayNextTurn reports whether the Effect is the exact chosen-card
// may-play shape and returns the chosen-card reference it acts on. The only
// lifetime taken is Duration$ UntilTheEndOfYourNextTurn; the one static is
// the MayPlay$ True grant over Card.ChosenCard in exile.
func (lc *lowerCtx) chosenMayPlayNextTurn(ps *params, ss []cards.Static) (lRef, bool) {
	if len(ss) != 1 || ss[0].Mode != "Continuous" || !ps.is("Duration", "UntilTheEndOfYourNextTurn") {
		return lRef{}, false
	}
	for _, k := range []string{"ValidTgts", "RememberObjects"} {
		if _, ok := ps.m[k]; ok {
			return lRef{}, false
		}
	}
	p := ss[0].Params
	if strings.TrimSpace(p["MayPlay"]) != "True" || strings.TrimSpace(p["Affected"]) != "Card.ChosenCard" ||
		strings.TrimSpace(p["AffectedZone"]) != "Exile" {
		return lRef{}, false
	}
	return lRef{Kind: rChosen}, true
}

// loseAbilitiesWhileHost lowers the Continuous static of a targeted Effect
// whose only content is RemoveAllAbilities$ over the remembered target,
// Duration$ AsLongAsInPlay with ForgetOnMoved$ Battlefield (The Wondrous
// Wasp). sp is the static's params with Mode and Description consumed.
func (lc *lowerCtx) loseAbilitiesWhileHost(ps, sp *params, obj lRef) error {
	if obj.Kind != rTarget || ps.str("RememberObjects") != "Targeted" || ps.str("Duration") != "AsLongAsInPlay" ||
		ps.str("ForgetOnMoved") != "Battlefield" || sp.str("Affected") != "Card.IsRemembered" ||
		sp.str("AffectedZone") != "Battlefield" || lc.ab == nil || lc.ab.Kind == akSpell {
		return refuse("Effect continuous shape")
	}
	ps.consume("Duration", "ForgetOnMoved")
	sp.consume("Affected", "AffectedZone", "RemoveAllAbilities")
	// This arm returns before effectTargetStatic's closing check, so an
	// extra key on the static must be refused here, not dropped.
	if left := sp.unread(); len(left) > 0 {
		return refuse("Effect static unread %v", left)
	}
	lc.emitOverride(obj, &lOverride{LoseAbilities: true, WhileHost: true}, "WhileHost")
	return nil
}
