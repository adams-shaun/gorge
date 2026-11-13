package kgen

import "testing"

// attackTaxForms: S:Mode$ CantAttackUnless lowers to kop.StatAttackTax when
// its Cost$ is generic mana (a literal, an inline Count$ or an SVar of the
// face; lower_attacktax.go), and every other shape gorge's attackPairCharge
// prices stays refused by name with its reason.
var attackTaxForms = []formCase{
	{card: "Propaganda", form: "Target$ You | Cost$ 2"},
	{card: "Baird, Steward of Argive", form: "Target$ You,Planeswalker.YouCtrl | Cost$ 1"},
	{card: "Onakke Oathkeeper", form: "Target$ Planeswalker.YouCtrl | Cost$ 1"},
	{card: "Sphere of Safety", form: "Cost$ X, X = Count$Valid Enchantment.YouCtrl"},
	{card: "Brainwash", form: "ValidCard$ Creature.AttachedBy, no Target$ | Cost$ 3"},
	{card: "Phyrexian Marauder", form: "ValidCard$ Card.Self | Cost$ Y, Y = its +1/+1 counters"},
	{card: "Norn's Annex", form: "Cost$ WP (a Phyrexian pip)", refused: `CantAttackUnless Cost$ "WP": a non-mana, coloured or Phyrexian attack cost is not lowered`},
	{card: "Reclamation", form: "Cost$ Sac<1/Land>", refused: `CantAttackUnless Cost$ "Sac<1/Land>"`},
	{card: "Hollow Warrior", form: "Cost$ tapXType<1/Creature.!attacking>", refused: `CantAttackUnless Cost$ "tapXType<1/Creature.!attacking>"`},
	{card: "Nils, Discipline Enforcer", form: "RememberingAttacker$ True | Cost$ X, X = the attacker's counters", refused: "a non-OppCtrl alternative", lir: "amount=CountersOn(TaxedAttacker,ALL)"},
	{card: "Archangel of Tithes", form: "IsPresent$ Card.Self+untapped beside a CantBlockUnless (blocktax_forms_test.go)"},
}

// TestAttackTaxForms pins the lowered and refused CantAttackUnless shapes.
func TestAttackTaxForms(t *testing.T) { checkForms(t, attackTaxForms) }
