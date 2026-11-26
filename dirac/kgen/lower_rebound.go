package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// K:Rebound (CR 702.88a): "If this spell was cast from your hand, instead of
// putting it into your graveyard as it resolves, exile it and, at the
// beginning of your next upkeep, you may cast this card from exile without
// paying its mana cost." gorge's cast_commit.go stamps FlagRebound on a
// hand-cast spell of a face carrying the keyword; moveResolvedOffStack
// exiles the resolved spell and registers the __kwReboundCast delayed
// trigger (Play Card.Self, ValidZone$ Exile, WithoutManaCost$, Optional$).
//
// The kernel owns the resolution-time half (dirac/kw_rebound.go: the exile
// and the your-next-upkeep delayed program). The body that program runs is
// lowered here as one latent ability: an optional, one-pick free cast of the
// source card in exile (OpCastFree, RefSelf), the same operand the
// exile-granted Play uses.

// lowerRebound lowers the printed K:Rebound line.
func (lc *lowerCtx) lowerRebound(node string) {
	if lc.out.Rebound != nil {
		return // CR 702.88c: multiple instances are redundant
	}
	if lc.out.Types&tPermanent != 0 || lc.out.Types&(tInstant|tSorcery) == 0 {
		lc.refuseNode(node, "Rebound on a permanent card")
		return
	}
	lc.out.Rebound = lc.reboundBody()
}

// reboundBody lowers the latent recast body: an optional, one-pick free cast
// of the source card in exile. The printed K:Rebound line keeps it as
// lCard.Rebound; a stack grant (Cast Through Time) keeps it on the carrier
// and points its StackKw static's GrantBody at it, so the upkeep program the
// kernel registers runs the carrier's body over the rebounding spell's exiled
// object (dirac/kw_rebound.go reboundPlan).
func (lc *lowerCtx) reboundBody() *lAbility {
	lat := lc.newLatent("ReboundCast")
	lc.withAbility(lat, func() error {
		lc.emit(lInstr{Op: "CastFree", Obj: lRef{Kind: rSelf}, C: zExile, CS: "Optional|One",
			Note: "CR 702.88a: the exiled card, free, optional (gorge __kwReboundCast)"})
		return nil
	})
	return lat
}

// refuseReboundCombos refuses Rebound beside anything else that moves the
// resolved spell: the kernel's exile at resolution is read against the plain
// instant/sorcery exit only (a flashback family cast, a multi-face card, a
// Paradigm).
func (lc *lowerCtx) refuseReboundCombos() {
	out := lc.out
	if out.Rebound == nil {
		return
	}
	if out.Flashback != nil || out.Harmonize != nil || out.Mayhem != nil || out.BeamMeUp != nil || out.Paradigm {
		lc.refuseNode("K:Rebound", "Rebound beside a graveyard cast route or Paradigm")
	}
	if faceLayout(lc.card, out.Token) != kop.FaceNone {
		lc.refuseNode("K:Rebound", "Rebound on a multi-face card")
	}
}
