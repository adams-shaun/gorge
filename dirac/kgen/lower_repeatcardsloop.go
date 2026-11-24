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
// Remembered), or that instruction behind one unless-payment ask (Cleansing's
// "destroy that land unless any player pays 1 life", UnlessCost$ with
// UnlessPayer$ Player): the plain loop is a resumable frame of the
// resolution's loop stack (dirac act_repeatcards.go repeatEachCardsFrame), so
// the ask is posed each pass and its answer carries the loop on. A body that
// asks anything else stays refused.
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
	n := len(lc.ab.Prog) - at - 1
	if n != 1 && !(n == 2 && lc.ab.Prog[at+1].Op == "UnlessPaid" && lc.ab.Prog[at+1].Skip == 1) {
		return refuse("RepeatEach cards loop body emitted %d instructions", n)
	}
	lc.ab.Prog[at].Skip = n
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
		// seat-exact: the Pump's recorded target controller, read after the
		// GainControl (lower_remembered.go pumpRememberOppController)
		return lRef{Kind: rResChosenPlayer}, true
	}
	return lc.capturedTriggerRef(v)
}
