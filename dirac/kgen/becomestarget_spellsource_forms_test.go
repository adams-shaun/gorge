package kgen

import "testing"

// TestBecomesTargetSpellSourceForms: the BecomesTarget trigger's spell source
// gate (ValidSource$ Spell, Spell.YouCtrl, Spell.OppCtrl, Spell.<Type>) over
// ValidTarget$ Card.Self lowers; gorge becomesTargetMatches reads it as an
// ordinary spell filter on the targeting stack object.
func TestBecomesTargetSpellSourceForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Labyrinth Guardian", form: "ValidSource$ Spell", lir: "trigger BecomesTarget"},
		{card: "Bonecrusher Giant", form: "ValidSource$ Spell (second printing)", lir: "trigger BecomesTarget"},
		{card: "Stormchaser Drake", form: "ValidSource$ Spell.YouCtrl", lir: "trigger BecomesTarget"},
		{card: "Opaline Sliver", form: "ValidSource$ Spell.OppCtrl", lir: "trigger BecomesTarget"},
		{card: "Fugitive Druid", form: "ValidSource$ Spell.Aura", lir: "trigger BecomesTarget"},
	})
}
