package kgen

import "testing"

// animateRemoveKeywordsForms: a bare Animate RemoveKeywords$ (no
// staticAbilities$) lowers as the layer-6 keyword-removal rider. Animate
// Dead and Dance of the Dead remove an "Enchant:" keyword, which has no
// kernel keyword bit (the Aura's attach restriction is not a keyword
// record), so they stay refused on that reason.
var animateRemoveKeywordsForms = []formCase{
	{card: "Ageless Sentinels", form: "Animate RemoveKeywords$ Defender (TriggeredBlockerLKICopy)", lir: "removekw=Defender"},
	{card: "Guardian of the Ages", form: "Animate RemoveKeywords$ Defender + Keywords$ Trample", lir: "removekw=Defender"},
	{card: "Walking Sponge", form: "Animate Defined$ Targeted RemoveKeywords$ Flying (until end of turn)", lir: "removekw=Flying"},
	{card: "Animate Dead", form: "RemoveKeywords$ Enchant:... (no kernel Aura restriction rewrite)", refused: `RemoveKeyword "Enchant:Creature.inZoneGraveyard`},
	{card: "Dance of the Dead", form: "RemoveKeywords$ Enchant:... (no kernel Aura restriction rewrite)", refused: `RemoveKeyword "Enchant:Creature.inZoneGraveyard`},
}

func TestAnimateRemoveKeywordsForms(t *testing.T) { checkForms(t, animateRemoveKeywordsForms) }
