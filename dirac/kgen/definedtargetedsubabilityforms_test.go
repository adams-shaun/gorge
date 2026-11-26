package kgen

import "testing"

// TestDefinedTargetedParentForms: Defined$ Targeted on a sub-ability with no
// target slot of its own names the parent ability's targets (Forge
// AbilityUtils.getDefinedObjects). A FlipCoin head declares its own ValidTgts$
// slot, which its branches read through the chain; a PutCounter union
// `Self & Targeted` declares its own slot once and reads it for the second
// part. Aleatory and Chaotic Strike get past Defined$ Targeted and stay
// refused on the spell's activation window (not lowered for a spell).
func TestDefinedTargetedParentForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Orcish Captain", form: "FlipCoin own slot; win/lose arm Pump Defined$ Targeted", lir: "Target0"},
		{card: "Aleatory", form: "FlipCoin own slot; win arm Pump Defined$ Targeted", refused: "unread params [ActivationAfterBlockers]"},
		{card: "Chaotic Strike", form: "FlipCoin own slot; win arm Pump Defined$ Targeted", refused: "unread params [ActivationAfterBlockers]"},
		{card: "Drillworks Mole", form: "PutCounter Defined$ Self & Targeted", lir: "Target0"},
		{card: "Beta Ray Bill", form: "trigger PutCounter Defined$ Self & Targeted", lir: "Target0"},
	})
}
