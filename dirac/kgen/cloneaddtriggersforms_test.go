package kgen

import "testing"

// TestCloneAddTriggersForms: an as-enters copy's AddTriggers$ (with the
// AddSVars$ its Execute$ names) lowers as a grant of a latent trigger to the copy
// (CR 707.9b); the carriers with IntoPlayTapped$ and Duration$ stay refused.
func TestCloneAddTriggersForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Phantasmal Image", form: "BecomesTarget Card.Self sacrifice grant", lir: "BecomeCopy"},
		{card: "Copycrook", form: "Attacks Card.Self connive grant", lir: "BecomeCopy"},
		{card: "Progenitor Mimic", form: "upkeep CopyPermanent grant", lir: "BecomeCopy"},
		// the grant itself lowers; its battlefield Clone body (SetColor$,
		// GainThisAbility$) is the clone-on-battlefield lowering's, so the card
		// stays refused by the body, not by the AddTriggers$ read
		{card: "Vesuvan Doppelganger", form: "upkeep Clone body (SetColor$ on the battlefield) refuses", refused: "Clone SetColor$ on the battlefield"},
		// its sameName trigger filter lowers now (the plain sameName is
		// PredSameNameSelf); the body still refuses, on its unread
		// IntoPlayTapped$ param
		{card: "Callidus Assassin", form: "IntoPlayTapped carrier stays refused", refused: "Clone unread params"},
		{card: "Vesuvan Shapeshifter", form: "Duration carrier stays refused", refused: "TurnFaceUp replacement shape"},
	})
}
