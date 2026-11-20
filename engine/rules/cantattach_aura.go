package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The S:Mode$ CantAttach Aura shapes (cards.CantAttachAuraShape): a live
// static forbids ATTACHING a second Aura to a bearer. Both shapes are
// attach-side only -- a restriction never un-attaches (Guardian Beast's
// printed "This effect doesn't remove Auras already attached" is the same
// rule stated per card, and no CR text sweeps an attached Aura for one) -- so
// there are exactly two readers and neither is the CR 704.5m detach SBA:
//
//   - restrictionBlocksTarget, the Aura spell's target census and the CR
//     608.2b resolution recheck (both call it), for the targeting half;
//   - auraEnchantCandidates, the CR 303.4f entry choice, for every entry that
//     does not resolve a spell: the choice must be a legal object or player
//     (CR 303.4f) and a bearer a CantAttach static forbids is not one.

// auraCantAttachBoard is the narrow surface cantAttachAuraBlocks reads: the
// game, the board static walk, the spec matcher and the one IsPresent$ gate.
type auraCantAttachBoard interface {
	Game() *state.Game
	activeStatics(mode string) []staticView
	matchesSpecFrom(spec string, id state.ObjID, you state.PlayerID, source state.ObjID) bool
	presentGate(sv staticView, spec string) bool
}

// cantAttachAuraBlocks reports whether a live Aura-shape CantAttach static
// forbids attaching the Aura aura to the bearer. Each static is classified by
// cards.CantAttachAuraShape (the shape's one home, reconstructed over the
// view's raw params), gated by IsPresent$ through presentGate (the one
// static IsPresent$ home), and then read exactly like cantAttachForbids: the
// aura-side spec (ValidCard$ Aura.Other, or ValidCardToTarget$ Aura.!Attached
// for Guardian Beast's shape) against aura and Target$ against bearer, both
// from the static's controller and source, so "EnchantedBy" names the bearer
// its own source enchants and "YouCtrl" the static's controller.
func cantAttachAuraBlocks(e auraCantAttachBoard, aura, bearer state.ObjID) bool {
	for _, sv := range e.activeStatics("CantAttach") {
		if !cards.CantAttachAuraShape(cards.Static{Mode: "CantAttach", Params: sv.Params}) {
			continue
		}
		if spec, ok := sv.Param(cards.PKIsPresent); ok && !e.presentGate(sv, spec) {
			continue
		}
		auraSpec := sv.ParamStr(cards.PKValidCard)
		if auraSpec == "" {
			auraSpec = sv.ParamStr(cards.PKValidCardToTarget)
		}
		if auraSpec == "" {
			continue
		}
		if e.matchesSpecFrom(auraSpec, aura, sv.Controller, sv.Source) &&
			e.matchesSpecFrom(sv.ParamStr(cards.PKTarget), bearer, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}
