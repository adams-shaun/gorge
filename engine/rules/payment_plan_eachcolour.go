package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// paymentPlanEachColourAlt is the payment-plan census's alternative for an
// each-colour ability on source id ("add one mana of each colour among the
// permanents you control [that match]": Bloom Tender, Faeburrow Elder): one
// fixed production, a mana of each colour among the matching permanents now
// (effects.EachColorAmongValidProduction, the resolution's own scan), not a
// choice (CR 106.1a, 605.1a). It reads only the permanents no planned step can
// remove, the reflected rule (paymentPlanReflectedObjectRemovable), so the
// production a step records is the one its live recomputation names. ok is
// false for any other ability, an amount other than one, or no colour.
func paymentPlanEachColourAlt(e *Engine, p state.PlayerID, id state.ObjID, ma *cards.SA, raw string, amt int32) (windowManaAlt, bool) {
	if amt != 1 || ma == nil || ma.API != "Mana" {
		return windowManaAlt{}, false
	}
	mask, ok := effects.EachColorAmongValidProduction(e, id, p, ma, raw, func(rid state.ObjID) bool {
		return paymentPlanReflectedObjectRemovable(e, rid)
	})
	if !ok || mask == 0 {
		return windowManaAlt{}, false
	}
	// ColorMask bit i is WUBRG index i, the counts' own layout.
	var counts [6]int32
	for i := range 5 {
		if mask&(1<<i) != 0 {
			counts[i] = 1
		}
	}
	return windowManaAlt{Ma: ma, Counts: counts, Amt: 1}, true
}
