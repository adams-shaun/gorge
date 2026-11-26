package kgen

import "testing"

// effectAltCostForms: the Effect-delivered AlternativeCost static (Marshland
// Bloodcaster's "Rather than pay the mana cost of the next spell you cast
// this turn, you may pay life equal to that spell's mana value"; gorge rules
// alternativeCosts' registry arm, engine/rules/statics_altcost.go), checked
// like effectflashForms. The grant ends at end of turn and on the
// controller's next completed cast (ForgetOnCast$ Card.YouCtrl), whether or
// not that cast used the alternative (CR 118.9, 119.4). On the base build
// the card is refused with "Effect static mode AlternativeCost without
// RememberObjects$" (lower_effectemblem.go's default arm) -- that refusal is
// this test's RED.
var effectAltCostForms = []formCase{
	{card: "Marshland Bloodcaster", form: "Effect AlternativeCost over every spell, ValidPlayer$ You, Cost$ PayLife<ConvertedManaCost> (an EOT emblem that ends on its controller's next cast)", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
}

func TestEffectAltCostLowers(t *testing.T) { checkForms(t, effectAltCostForms) }
