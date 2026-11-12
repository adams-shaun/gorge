package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// optionalMoveShape reports whether a known-origin ChangeZone Optional$ True
// move is one an OpMayAsk guard mirrors exactly, and which guard:
//   - the source leaving the battlefield to its owner's hand (Intimidation
//     Campaign, Mistbreath Elder): kop.MayGuardMove, gorge's
//     changeZoneOptionalConfirm over the one live object its loop keeps; or
//   - the card a one-card looking Peek just remembered, moved from the library
//     to the battlefield (Into the Wilds): kop.MayGuardMoveRemembered, gorge's
//     Defined$ library fetch, which asks once while the remembered set holds
//     an object and never for an empty one; or
//   - the card the Dig just remembered, exiled, then optionally put onto the
//     battlefield (Primal Surge): the same guard. The move carries
//     MvForgetOther, so the kernel also forgets the set on a declined ask,
//     as gorge's forgetOther does before the confirmation.
//
// Another object set or origin has its own skip rules (changeZoneMoveSkip,
// sourceReturnedSincePush, a player target) and stays refused.
func (lc *lowerCtx) optionalMoveShape(obj lRef, origin int, in lInstr, override *lOverride, self bool) (guard int, ok bool) {
	if override != nil || self || in.C != 0 && in.C != zBattlefield || in.A.Kind != 0 {
		return 0, false
	}
	switch {
	case obj.Kind == rSelf && origin == zBattlefield && in.Op == "Bounce" && in.C == 0 && in.CS == "from=Battlefield":
		return kop.MayGuardMove, true
	case obj.Kind == rRemembered && origin == zLibrary && in.Op == "Return" && in.C == zBattlefield && in.CS == "from=Library":
		return kop.MayGuardMoveRemembered, lc.afterOneCardPeek()
	case obj.Kind == rRemembered && origin == zExile && in.Op == "Return" && in.C == zBattlefield && in.CS == "Remember|ForgetOther|from=Exile":
		return kop.MayGuardMoveRemembered, true
	case obj.Kind == rImprinted && origin == zExile && in.Op == "Bounce" && in.C == 0 && in.CS == "from=Exile":
		// "you may put that card into your hand" over the card the Dig just
		// imprinted (Tainted Pact): the guard is the chosen object's
		// existence where it now lies, as the library Peek's arm
		return kop.MayGuardMoveRemembered, true
	}
	return 0, false
}

// afterOneCardPeek reports whether the instruction emitted last is a looking
// Peek of one card that remembers it (PeekAndReveal PeekAmount$ 1 NoReveal$
// True RememberPeeked$ True): the remembered set then holds only that card.
func (lc *lowerCtx) afterOneCardPeek() bool {
	n := len(lc.ab.Prog)
	if n == 0 {
		return false
	}
	p := &lc.ab.Prog[n-1]
	fl := flagsOf(p.CS)
	_, rem := fl["Remember"]
	_, pub := fl["Public"]
	return p.Op == "Peek" && rem && !pub && isConstN(p.A, 1)
}
