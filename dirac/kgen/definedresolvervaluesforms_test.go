package kgen

import "testing"

var definedResolverValuesForms = []formCase{
	// `A & B` over two players lowers the node once per part
	{card: "Karazikar, the Eye Tyrant", form: "Draw Defined$ TriggeredAttackingPlayer & You", lir: "Draw obj=TriggeredPlayer"},
	{card: "Cait, Cage Brawler", form: "Draw lowers beside its still-refused Discard join", lir: "Draw obj=Defending", refused: "DB$ Discard: Defined$ \"TriggeredDefendingPlayer & You\""},
	// EnchantedController: the live controller of the enchanted permanent
	{card: "Power Taint", form: "UnlessPayer$ EnchantedController", lir: "UnlessPaid obj=EnchantedCtrl"},
	{card: "Gremlin Infestation", form: "DealDamage Defined$ EnchantedController", lir: "EnchantedCtrl"},
	// kept refused: the kernel records no object for these referents
	{card: "Venerated Loxodon", form: "Defined$ Convoked (the kernel keeps only the convoke count)", refused: `Defined$ "Convoked"`},
	{card: "Storm, Windrider", form: "Defined$ TriggeredSpellAbilityTargets (no kernel ref to a cast spell's targets)", refused: `Defined$ "TriggeredSpellAbilityTargets"`},
	{card: "Energybending", form: "RememberObjects$ Valid <filter> (no emblem capture of a filtered set)", refused: `Defined$ "Valid Land.YouCtrl"`},
	{card: "Saskia the Unyielding", form: "Defined$ ChosenPlayer after ChoosePlayer Choices$ Player (the kernel chooses opponents only)", refused: `Defined$ "ChosenPlayer"`},
}

func TestDefinedResolverValuesForms(t *testing.T) {
	checkForms(t, definedResolverValuesForms)
}
