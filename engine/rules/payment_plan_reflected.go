package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// paymentPlanReflectedAlt is the payment-plan census's alternative for a
// pay.PaymentPlanReflectedShape ability on source id (Exotic Orchard): the
// colours the resolution's own candidate walk names right now, as a
// choice-shaped alternative the planner expands into one concrete colour
// each (pay.PaymentPlanUnitAlternatives). ok is false when the ability is
// not that shape or reflects no colour.
func paymentPlanReflectedAlt(e *Engine, p state.PlayerID, id state.ObjID, ma *cards.SA, amt int32) (windowManaAlt, bool) {
	if !pay.PaymentPlanReflectedShape(ma) {
		return windowManaAlt{}, false
	}
	var svars map[string]string
	if o := e.Game().Obj(id); o != nil && o.Face() != nil {
		svars = o.Face().SVars
	}
	ctx := effects.NewCtxPtr(id, p, effects.CtxInit{SVars: svars})
	// Only objects no planned step can remove contribute: an own reflected
	// land whose mana ability sacrifices or returns it could be gone before
	// the reflected step runs (the executor would then fall back mid-payment).
	var kept []state.ObjID
	for _, rid := range effects.ManaReflectedObjects(e, ctx, ma) {
		if !paymentPlanReflectedObjectRemovable(e, rid) {
			kept = append(kept, rid)
		}
	}
	counts, ok := pay.PaymentPlanReflectedCounts(effects.ManaReflectedFold(e, ma, kept))
	if !ok {
		return windowManaAlt{}, false
	}
	return windowManaAlt{Ma: ma, Counts: counts, Amt: amt, Any: true}, true
}

// paymentPlanReflectedProductions is a reflected ability's productions read
// from its census alternative (paymentPlanReflectedAlt) in place of
// pay.PaymentPlanProductions: a ManaReflected body has no Produced$, so the
// pay read names no production and the bound never added a tap trigger's
// extra mana (Overgrowth's additional {G}{G} beside a Reflecting Pool, CR
// 605.1b) to it. Each candidate colour is one production, the choices the
// planner expands the unit into. The source-interference verdict keeps the
// pay read: a reflected source stays an ordinary unit beside an unvetted Taps
// watcher.
func paymentPlanReflectedProductions(alt windowManaAlt) []string {
	var out []string
	for i, n := range alt.Counts {
		if n > 0 {
			out = append(out, string(cards.ManaSymbol(i)))
		}
	}
	return out
}

// addPotentialReflected folds a pay.PaymentPlanReflectedShape ability into
// PotentialMana's bound as the colours the payment-plan census offers for it
// (each candidate colour at the ability's amount), and reports whether it
// did. The ability's text alone reads as a blank Produced$ -- the executor's
// colourless default for AB$ Mana -- which would drop every coloured cast
// only an Exotic Orchard pays from the planner's candidate walk.
func addPotentialReflected(e *Engine, m *state.Mana, p state.PlayerID, id state.ObjID, ma *cards.SA) bool {
	alt, ok := paymentPlanReflectedAlt(e, p, id, ma, 1)
	if !ok {
		return false
	}
	for i, n := range alt.Mana() {
		m[i] = saturatingPotentialMana(m[i], n)
	}
	return true
}

// addPotentialReflectedAmount colours the bound for a ManaReflected ability
// whose Amount$ keeps it out of the planner's shape (Incubation Druid: "one
// mana of any type that a land you control could produce", three with a
// counter). The caller still folds the ability's blank-Produced$ {C} stand-in;
// this adds each colour the reflection names right now, unbounded as the
// amount is indeterminate. CR 106.7 prices what the lands "could produce"
// from their printed mana abilities, tapped or not, so a live druid beside
// tapped Swamps funds {B} (a Produce read only; Is and Produced are left to
// the existing shape).
func addPotentialReflectedAmount(e *Engine, m *state.Mana, p state.PlayerID, id state.ObjID, ma *cards.SA) {
	if ma == nil || ma.APIKind() != cards.APIManaReflected {
		return
	}
	mr := effects.ManaReflectedOf(ma)
	if !mr.Amount.Present || mr.Produced != "" || mr.ReflectProperty != "Produce" {
		return
	}
	var svars map[string]string
	if o := e.Game().Obj(id); o != nil && o.Face() != nil {
		svars = o.Face().SVars
	}
	ctx := effects.NewCtxPtr(id, p, effects.CtxInit{SVars: svars})
	for _, sym := range effects.ManaReflectedCandidates(e, ctx, ma) {
		if i := state.ManaIndex(sym[0]); i >= 0 {
			m[i] = saturatingPotentialMana(m[i], potentialUnbounded)
		}
	}
}

// setPaymentPlanUnitAlt puts alt into source id's unit (units[idx], idx < 0:
// none yet): it replaces the alternative the shared census already listed
// for the same ability (its colourless stand-in for a reflected ability),
// else appends.
func setPaymentPlanUnitAlt(units []windowManaUnit, idx int, id state.ObjID, alt windowManaAlt) ([]windowManaUnit, int) {
	if idx < 0 {
		units = append(units, windowManaUnit{ID: id})
		idx = len(units) - 1
	}
	for k := range units[idx].Alts {
		if units[idx].Alts[k].Ma == alt.Ma {
			units[idx].Alts[k] = alt
			return units, idx
		}
	}
	units[idx].Alts = append(units[idx].Alts, alt)
	return units, idx
}

// dropPaymentPlanUnitAlt removes the shared census's blank colourless stand-in
// for a reflected ability on source id when the reflection resolves no symbol
// (idx < 0: the census listed no unit, so there is nothing to drop). Without
// it the stand-in's {C} would survive as a concrete choice even though the
// reflected ability cannot produce {C} from the current board.
func dropPaymentPlanUnitAlt(units []windowManaUnit, idx int, id state.ObjID, ma *cards.SA) []windowManaUnit {
	if idx < 0 || idx >= len(units) || units[idx].ID != id {
		return units
	}
	alts := units[idx].Alts[:0]
	for _, a := range units[idx].Alts {
		if a.Ma != ma {
			alts = append(alts, a)
		}
	}
	units[idx].Alts = alts
	return units
}

// paymentPlanReflectedObjectRemovable reports whether a payment step could
// remove battlefield object rid before a later reflected step runs: one of
// its own mana abilities classifies as a plannable (non-deferred) step whose
// consequence sacrifices it or returns it to hand. Objects outside the
// battlefield (an imprinted card, a graveyard or exiled card) have no
// plannable mana ability and are never removed by a step.
func paymentPlanReflectedObjectRemovable(e *Engine, rid state.ObjID) bool {
	o := e.Game().Obj(rid)
	if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
		return false
	}
	for _, ma := range o.Face().ManaAbilities() {
		tier, c, _ := pay.PaymentPlanAbilityTier(asPayer(e), o.Controller, rid, ma)
		if tier != pay.TierDeferred && (c.Sacrifice || c.ReturnToHand) {
			return true
		}
	}
	return false
}
