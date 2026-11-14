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
	if pc == nil || pc.PaymentFallback == nil {
		return owed, false
	}
	return pay.PaymentOwed(e.castPaymentMana(pc), e.G.Players[pc.player].Pool), true
}
