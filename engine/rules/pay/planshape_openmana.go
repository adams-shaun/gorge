package pay

import (
	"maps"
	"slices"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects/params"
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
	body = paymentPlanStaticTapManaBody(f, t, "Mana")
	if body == nil {
		return nil, false, false
	}
	for _, k := range slices.Sorted(maps.Keys(body.Params)) {
		switch k {
		case "API", "Produced", "Amount", "Defined", "SpellDescription", "StackDescription":
		default:
			return nil, false, false
		}
	}
	switch ref := params.DefinedRefOf(body); {
	case ref.Kind == params.RefAbsent || ref.Kind == params.RefYou:
	case ref.Text == "TriggeredCardController":
		toTapped = true
	default:
		return nil, false, false
	}
	return body, toTapped, true
}
