package rules

import (
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// PendingCastOwed is the mana the pending cast or activation's CR 601.2g
// payment still owes after its floating pool, by colour: the Owed a manual
// payment window carries as ManaPayment. A payment window a plan fell back to
// carries no ManaPayment and neither does the colour ask of a dual land
// activated from it, so a driver answering that ask reads the owed colour
// here. ok is false when no cast is in flight or its payment plan has not
// fallen back to the manual window (a planned payment's own asks are the
// plan's).
func PendingCastOwed(e *Engine) (owed decision.PaymentCost, ok bool) {
	pc := e.cast
	if pc == nil {
		return owed, false
	}
	if pc.PaymentFallback == nil {
		if len(pc.Convoke) == 0 || pc.isAbility() {
			return owed, false
		}
		// A cast that elected Convoke/Improvise poses the classic window with
		// no fallback and no ManaPayment: the ask lists what the composed
		// cost (before the elected creatures' share, CR 702.51a) still owes
		// after the pool, which is what the window's sources are offered by
		// (paymentUsefulColours reads manaToPay).
		mana := e.manaToPay(pc)
		mana.Generic -= int32(len(pc.delve))
		if mana.Generic < 0 {
			mana.Generic = 0
		}
		return pay.PaymentOwed(mana, e.G.Players[pc.player].Pool), true
	}
	return pay.PaymentOwed(e.castPaymentMana(pc), e.G.Players[pc.player].Pool), true
}
