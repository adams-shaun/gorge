package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// lockedGrantAffects is the spec a locked grant carries: the frozen ids ride
// the registration's Remembered, which the layer walk binds for it.
const lockedGrantAffects = "Permanent.IsRemembered"

// lockEffectGrant freezes the affected set of an Effect-delivered layer grant
// whose Affected$ spec reads a layer-derived power or toughness, as the effect
// begins (CR 611.2c: the set of objects a resolution-generated continuous
// effect affects is determined when it begins and never changes, so a
// creature that later shrinks into Temur Charm's "power 3 or less" gate is not
// in it, and one that grows out of it stays). It rewrites ce to
// `Permanent.IsRemembered` over the matched ids, the seam the layer walk
// already binds, and reports whether the effect still registers: a set that
// was empty at the beginning registers nothing.
//
// A grant keeps its dynamic spec (true, ce untouched) when it reads no P/T
// (this lock is gated to exactly the specs that can change membership between
// the effect's resolution and a later read), when it already names the
// Effect's own remembered set, or when its AffectedZone is not the
// battlefield. Printed statics never reach here. Seats are walked in
// AliveFrom(0) order and each battlefield in zone order, never a map.
func lockEffectGrant(h Host, c *Ctx, ce *state.ContinuousEffect) bool {
	if len(ce.Remembered) > 0 || ce.Affects == "" || strings.Contains(ce.Affects, "IsRemembered") {
		return true
	}
	if z := ce.AffectedZone; z != "" && !strings.EqualFold(z, "Battlefield") {
		return true
	}
	if !SpecReadsPT(ce.Affects) {
		return true
	}
	gm := h.Game()
	var ids []state.ObjID
	for _, p := range gm.AliveFrom(0) {
		for _, id := range gm.Zone(state.ZBattlefield, p) {
			if MatchesSpecCtx(gm, ce.Affects, id, c.SpecContext(c.Controller)) {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return false
	}
	ce.Affects, ce.Remembered = lockedGrantAffects, ids
	if ce.ForgetOnMoved == "" {
		ce.ForgetOnMoved = "Battlefield" // a creature that leaves is a new object (CR 400.7)
	}
	return true
}
