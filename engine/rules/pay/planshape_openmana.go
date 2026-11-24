package pay

import (
	"github.com/adams-shaun/gorge/engine/cards"
)

// PaymentPlanOpenTapMana classifies a `Mode$ TapsForMana | Static$ True`
// trigger of face f whose single `DB$ Mana` body adds mana the planner cannot
// fold into a source's production because the player picks its colour at the
// tap (Dawn's Reflection's two mana "in any combination of colors", Fertile
// Ground's one mana of any color: CR 605.3b). The source stays deferred; the
// caller only reads the body to bound what a tap may add (potentialTapExtra).
// toTapped records Defined$ TriggeredCardController. It is closed-world as
// PaymentPlanFoldableTapMana, and refuses what that one folds.
func PaymentPlanOpenTapMana(f *cards.Face, t cards.Trigger) (body *cards.SA, toTapped, ok bool) {
	if _, folds := PaymentPlanFoldableTapMana(f, t); folds {
		return nil, false, false
	}
	return paymentPlanClosedTapManaBody(f, t)
}
