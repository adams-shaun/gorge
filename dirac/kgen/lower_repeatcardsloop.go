package kgen

import (
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// repeatEachCardsLoop lowers api:RepeatEach over cards without
// ChangeZoneTable$ True (gorge effRepeatEach with zoneTable false): a plain
// per-card loop, the body run once per subject with Remembered bound to that
// subject for its pass (cc.Remembered = [card]; RefEach, read live). Gorge
// opens no zone batch around the loop, so each pass's move, if any, is its
// own; the kernel's loop then opens neither the batch nor the entry hold.
// The body is one instruction over the repeated card (Defined$/Object$
// Remembered) that never asks: the kernel's loop runs the body inline and
// cannot resume an ask from inside it.
func (lc *lowerCtx) repeatEachCardsLoop(in lInstr, body *cards.SA) error {
	if body.Sub != nil {
		return refuse("RepeatEach cards loop body %s with a SubAbility$", body.API)
	}
	in.C = kop.RepeatPlain
	at, nt := len(lc.ab.Prog), len(lc.ab.Targets)
	lc.emit(in)
	lc.repeatCard++
	err := lc.chain(body, nil)
	lc.repeatCard--
	if err != nil {
		return err
	}
	if len(lc.ab.Targets) != nt {
		return refuse("RepeatEach cards loop body with a target")
	}
	if len(lc.ab.Prog) != at+2 {
		return refuse("RepeatEach cards loop body emitted %d instructions", len(lc.ab.Prog)-at-1)
	}
	lc.ab.Prog[at].Skip = 1
	return nil
}

// rememberedAlias is what Defined$ Remembered reads when it is not the
// resolution's remembered set: the repeated card of a RepeatEach-over-cards
// body (repeatEachCardsLoop), the Pump-remembered target's controller
// (lower_remembered.go), or the captured trigger source
// (lower_effeottrig.go capturedTriggerRef).
func (lc *lowerCtx) rememberedAlias(v string) (lRef, bool) {
	switch {
	case v == "Remembered" && lc.repeatCard > 0:
		return lRef{Kind: rEach}, true
	case v == "Remembered" && lc.remOppLink():
		// seat-exact: the Pump's target controller, not the one opponent
		return lRef{Kind: rTargetController, N: lc.remOppSlot}, true
	}
	return lc.capturedTriggerRef(v)
}
