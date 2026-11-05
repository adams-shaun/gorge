package kgen

import "github.com/adams-shaun/gorge/engine/cards"

// discardOppToBattlefield lowers the Wilt-Leaf Liege shape: a hand card
// discarded by an opponent's spell or ability enters the battlefield instead.
// Exactly this parameter set; any other empty-Destination Moved line keeps
// the refusal (false, nil).
func (lc *lowerCtx) discardOppToBattlefield(rp *params, w *cards.SA, rl *lRepl) (bool, error) {
	if rp.str("Destination") != "" || rp.str("ValidCard") != "Card.Self" || rp.str("ActiveZones") != "Hand" ||
		rp.str("Discard") != "True" || rp.str("EffectOnly") != "True" || rp.str("ValidCause") != "SpellAbility.OppCtrl" ||
		!withIs(w, "ChangeZone", map[string]string{"Defined": "ReplacedCard", "Origin": "Hand", "Destination": "Battlefield"}) {
		return false, nil
	}
	rl.Kind = "DiscardOppToBattlefield"
	return true, nil
}
