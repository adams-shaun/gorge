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
//
// The same grant covers a cycling activation (New Perspectives' "As long as you
// have seven or more cards in hand, you may pay {0} rather than pay cycling
// costs", ValidSA$ Activated.Cycling): the static's Cost$ replaces the whole
// printed cost, its own Discard<1/CARDNAME> being the discard cycling's
// printed cost already carried, so the alternative drops the mana and any
// Pay-life part and keeps the discard. TypeCycling is a variant of cycling
// (cyclingKeyword), so it takes the same alternative.
func (e *Engine) abilityAlternateCostFor(p state.PlayerID, src state.ObjID, ab *cards.SA) (Cost, bool) {
	if c, ok := e.abilityAlternateCost(ab); ok {
		return c, true
	}
	kw, ok := staticAltCostKind(ab)
	if !ok {
		return Cost{}, false
	}
	for _, sv := range e.activeStatics("AlternativeCost") {
		if !staticAltCostScopesAbility(sv, kw, p) && !e.staticAltCostScopesCrew(sv, kw, p, src) {
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

// staticAltCostKind is the ValidSA$ Activated.<kind> an AlternativeCost static
// names for ab: the attach-cost SA's own keyword (Equip, Fortify), Crew for the
// minted crew ability (a Vehicle's own alternative, staticAltCostScopesCrew), or Cycling
// for a cycling / typecycling body (cyclingKeywordCodes). ok is false for every
// other activation.
func staticAltCostKind(ab *cards.SA) (string, bool) {
	if ab == nil {
		return "", false
	}
	kw := strings.TrimSpace(ab.ParamStr(cards.PKKeyword))
	if isAttachCostSA(ab) || isCrewSA(ab) {
		return kw, true
	}
	if cyclingKeywordCodes.Code(kw) == cyclingKeywordCycling {
		return "Cycling", true
	}
	return "", false
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

// isCrewSA reports whether ab is the crew ability engine/cards/kw_crew.go
// mints (Keyword$ Crew, CR 702.122).
func isCrewSA(ab *cards.SA) bool {
	return ab != nil && strings.TrimSpace(ab.ParamStr(cards.PKKeyword)) == "Crew"
}

// staticAltCostScopesCrew is staticAltCostScopesAbility's twin for the
// Vehicle's own crew alternative (Heart of Kiran: ValidSA$
// Activated.Crew+Vehicle, ValidCard$ Card.Self, ValidPlayer$ You): the static
// grants its alternative only to the crew ability of the permanent that
// carries it (CR 118.9), activated by its controller. Any other rider shape
// fails closed.
func (e *Engine) staticAltCostScopesCrew(sv staticView, kw string, p state.PlayerID, src state.ObjID) bool {
	if kw != "Crew" || sv.Source != src || p != sv.Controller {
		return false
	}
	if vc, has := sv.Param(cards.PKValidCard); !has || strings.TrimSpace(vc) != "Card.Self" {
		return false
	}
	if !strings.EqualFold(strings.TrimSpace(sv.ParamStr(cards.PKValidPlayer)), "You") {
		return false
	}
	if strings.TrimSpace(sv.ParamStr(cards.PKValidSA)) != "Activated.Crew+Vehicle" {
		return false
	}
	o := e.G.Obj(src)
	return o != nil && e.hasType(o, "Vehicle")
}
