package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// abilityAlternateCostFor is abilityAlternateCost widened to the
// battlefield-static grant of an attach-cost alternative: Kíli the
// Resourceful's "As long as you have an enduring story, you may pay {0}
// rather than pay the equip cost of the first equip ability you activate each
// turn" (an AlternativeCost static of {0} for your equip abilities, gated on
// the enduring-story condition and an SVar count of zero).
// The ability's own AlternateCost$ rider wins (it is printed on the ability
// itself); otherwise the first applying static's Cost$ is the one alternative
// (AltCostIndex 1), offered and charged through the same option the rider
// uses (CR 118.9: an alternative cost replaces the printed one). p is the
// activator.
func (e *Engine) abilityAlternateCostFor(p state.PlayerID, ab *cards.SA) (Cost, bool) {
	if c, ok := e.abilityAlternateCost(ab); ok {
		return c, true
	}
	if !isAttachCostSA(ab) {
		return Cost{}, false
	}
	kw := strings.TrimSpace(ab.ParamStr(cards.PKKeyword))
	for _, sv := range e.activeStatics("AlternativeCost") {
		if !staticAltCostScopesAbility(sv, kw, p) {
			continue
		}
		if !e.continuousConditionHolds(sv) || !e.checkSVarHolds(sv) {
			continue
		}
		c := e.parseCost(sv.ParamStr(cards.PKCost))
		if len(c.Unknown) > 0 {
			continue
		}
		return c, true
	}
	return Cost{}, false
}

// staticAltCostScopesAbility reports whether an AlternativeCost static grants
// its alternative to the activator p's <kw> ability: no ValidCard$ (a
// spell-scoped grant), ValidSA$ exactly Activated.<kw>, and a ValidPlayer$
// that is absent or You (relative to the static's controller). Any other
// rider shape fails closed.
func staticAltCostScopesAbility(sv staticView, kw string, p state.PlayerID) bool {
	if _, has := sv.Param(cards.PKValidCard); has || kw == "" {
		return false
	}
	kind, ok := strings.CutPrefix(strings.TrimSpace(sv.ParamStr(cards.PKValidSA)), "Activated.")
	if !ok || !strings.EqualFold(kind, kw) {
		return false
	}
	switch vp := strings.TrimSpace(sv.ParamStr(cards.PKValidPlayer)); {
	case vp == "":
	case strings.EqualFold(vp, "You"):
		if p != sv.Controller {
			return false
		}
	default:
		return false
	}
	return true
}
