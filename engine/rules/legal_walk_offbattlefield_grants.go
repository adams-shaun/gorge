package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// grantedAnchorZoneOK is the shared zone constraint for both offering an
// SVar-anchored granted activation and accepting its priority answer.
func grantedAnchorZoneOK(o *state.Object, ab *cards.SA) bool {
	return o != nil && o.Face() != nil && ab != nil && abilityZoneOK(ab, o.Zone)
}

func isMaxSpeedCondition(condition string) bool { return condition == "MaxSpeed" }

func maxSpeedBoastHolds(w *legalWalk, id state.ObjID, ab *cards.SA, svar string) bool {
	return !strings.EqualFold(strings.TrimSpace(ab.ParamStr(cards.PKBoast)), "True") ||
		w.e.boastGateOK(id, -1, svar)
}

// offBattlefieldGrantedWalk offers the supported MaxSpeed AddAbility carriers
// collected from non-battlefield EffectZone$ memberships. Battlefield carriers
// stay with the existing layer/max-speed offer loops, avoiding duplicates.
func offBattlefieldGrantedWalk(w *legalWalk) {
	exileGrantedWalk(w)
	e, p := w.e, w.p
	out := &w.out
	for _, carrier := range w.actionStatics.addAbilityContinuous() {
		if carrier.Controller != p || !isMaxSpeedCondition(carrier.ParamStr(cards.PKCondition)) ||
			e.G.Players[carrier.Controller].Speed < maxSpeed {
			continue
		}
		o := e.G.Obj(carrier.Source)
		if o == nil || o.Zone == state.ZBattlefield || o.Face() == nil ||
			!e.matchesSpec(carrier.ParamStr(cards.PKAffected), carrier.Source,
				e.specCtx(carrier.Source, carrier.Controller)) {
			continue
		}
		sv := strings.TrimSpace(carrier.ParamStr(cards.PKAddAbility))
		ab := e.grantedSAFrom(carrier.Source, carrier.Source, sv)
		if !grantedAnchorZoneOK(o, ab) || cards.IsManaAbilitySA(ab) {
			continue
		}
		if w.abilityRestricted(p, carrier.Source, ab) || e.castSuppressed(p, carrier.Source) {
			continue
		}
		cost := e.abilityCompiledCost(ab).Cost
		if pay.ActivationTapCostUnavailable(o, &cost) ||
			pay.TapCostSick(asPayer(e), carrier.Source, &cost, activatesAsIfHaste(e, carrier.Source)) ||
			!w.offerCastable(p, carrier.Source, cost, abilityScope(ab), true) ||
			!e.abilityTargetsAvailable(p, carrier.Source, ab) ||
			!e.abilityPresentHolds(p, carrier.Source, ab) || !e.adaptGateOK(carrier.Source, ab) {
			continue
		}
		if !maxSpeedBoastHolds(w, carrier.Source, ab, sv) {
			continue
		}
		*out = append(*out, decision.Option{Index: len(*out), Kind: "granted",
			Label: o.Face().Name + ": " + ab.ParamStr(cards.PKSpellDescription),
			Obj:   carrier.Source, SVar: sv})
	}
}

// exileGrantedWalk offers the activated abilities a layer-6 grant gives the
// cards in p's exile (CR 613.1f, 602.2): Q, Stacking the Deck's Animate gives
// the card it exiles "Exile a card from your hand: Create a copy of this card.
// You may cast the copy. Activate only during your turn and only if this card
// is exiled", a body whose ActivationZone$ names Exile. The battlefield's
// granted loop never reaches these cards, and the printed exile walk reads
// the printed face only, so without this walk the ability was granted and
// never offered. The gates are the battlefield granted loop's, minus the ones
// that read a permanent (phasing, {T}, loyalty): the body must name the
// carrier's zone (grantedAnchorZoneOK), and a mana ability or a gained
// foreign-face ability stays with its own walk.
func exileGrantedWalk(w *legalWalk) {
	e, p := w.e, w.p
	if !e.activeSummaryOf(e.active()).hasGrants {
		return
	}
	out := &w.out
	for _, id := range e.G.Zone(state.ZExile, p) {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil || o.FaceDown {
			continue
		}
		for _, ga := range e.grantedAbilities(p, id) {
			ab := ga.SA
			if ga.Gained || !grantedAnchorZoneOK(o, ab) || cards.IsManaAbilitySA(ab) {
				continue
			}
			if ab.ParamStr(cards.PKSorcerySpeed) == "True" && !w.sorcery {
				continue
			}
			if !e.activatorAllows(p, id, ab) || !e.activationPhasesOK(p, ab) {
				continue
			}
			if w.abilityRestricted(p, id, ab) || e.castSuppressed(p, id) {
				continue
			}
			cost := e.abilityCompiledCost(ab).Cost
			bindGrantedCostReferents(&cost, ga.Source)
			if !w.offerCastable(p, id, cost, abilityScope(ab), true) ||
				!e.abilityTargetsAvailable(p, id, ab) ||
				!e.abilityPresentHolds(p, id, ab) || !e.adaptGateOK(id, ab) {
				continue
			}
			if !maxSpeedBoastHolds(w, id, ab, ga.SVar) ||
				e.activationLimitBlocked(p, id, ab, -1, ga.SVar, 0) || e.grantedSAFrom(ga.Source, id, ga.SVar) == nil {
				continue
			}
			*out = append(*out, decision.Option{Index: len(*out), Kind: "ability",
				Label: o.Face().Name + ": " + ab.ParamStr(cards.PKSpellDescription), Obj: id, SVar: ga.SVar,
				GrantSource: ga.Source, Attach: ab.API == "Attach"})
		}
	}
}
