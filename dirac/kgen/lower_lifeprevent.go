package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// lifePreventStatic lowers the printed blanket gain-life prevention (Sulfuric
// Vortex: R:Event$ GainLife | ActiveZones$ Battlefield | Prevent$ True, "if a
// player would gain life, that player gains no life instead") as the
// CantGainLife static marked as the replacement form (Amount 1, kernel
// Static.A 1). Alone, gorge's applyLifeReplacements swallows the gain of
// every player when a Prevent$ True GainLife line is active
// (lifeReplPrevents), the same result as its CantGainLife restriction
// (lifeGainForbidden). But the line is a replacement, not a restriction: it
// competes with a gain-doubling or plus-N replacement in the CR 616.1 order
// ask (replacement_life.go lifeReplacementsCommute), which the kernel's
// static does not pose, so the gain site stalls on that competition
// (static_lifeprevent.go). A ValidPlayer$ scope, a ReplaceWith$ body or any
// other key refuses by name. false: not this shape.
func (lc *lowerCtx) lifePreventStatic(rp *params, w *cards.SA) (bool, error) {
	if w != nil || !strings.EqualFold(rp.peek("Prevent"), "True") {
		return false, nil
	}
	if rp.str("ActiveZones") != "Battlefield" {
		return true, refuse("repl-hard: GainLife prevention ActiveZones %q", rp.str("ActiveZones"))
	}
	rp.consume("Prevent", "AILogic")
	if left := rp.unread(); len(left) > 0 {
		return true, refuse("repl-hard: GainLife prevention params %v", left)
	}
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "CantGainLife", Amount: cst(1)})
	return true, nil
}
