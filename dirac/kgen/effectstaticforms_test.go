package kgen

import "testing"

// effectstaticformsForms: lowerings, one carrier
// per class plus the shapes that stay refused, checked like paramreadformsForms.
var effectstaticformsForms = []formCase{
	{card: "Goblin Diplomats", form: "Effect MustAttack static, no RememberObjects$ (gorge effEffect StaticMustAttack: UntilEOT emblem static)", lir: "0 CreateEmblem obj=You\n  emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Dread Charge", form: "Effect CantBlockBy static, no RememberObjects$ (gorge restriction case, absentDurationMeansThisTurn)", lir: "0 CreateEmblem obj=You\n  emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Walking Desecration", form: "Effect MustAttack static over Creature.ChosenType stays refused (an emblem has no source to read the choice from)", refused: "Effect static filter {types=0x0 ChosenType} reads the effect's chosen type or colour"},
	{card: "Alchemist's Refuge", form: "Effect CastWithFlash spell grant (gorge SpellFlashParamsReadable: ValidSA$ Spell, Caster$ You; an EOT emblem)", lir: "emblem[0] trigger=0 statics=1 until=EOT"},
	{card: "Cosmotronic Wave", form: "Effect CantBlock stays refused (gorge effectUnsupportedStatic)", refused: "Effect static mode CantBlock without RememberObjects$"},
	{card: "Reminisce", form: "ChangeZoneAll Shuffle$ of a target player's graveyard (gorge: the scoped library shuffles even when empty)", lir: "0 ChangeZoneAll obj=Target0 C=4 CS=from=Graveyard F={types=0x0} // only the target player's zone\n     1 Shuffle obj=Target0"},
	{card: "Feldon's Cane", form: "ChangeZoneAll Shuffle$ of your own graveyard (gorge: you shuffle even when it is empty)", lir: "0 ChangeZoneAll C=4 CS=from=Graveyard F={types=0x0 owner=1}\n     1 Shuffle obj=You"},
	{card: "Nihil Spellbomb", form: "ChangeZoneAll Shuffle$ into exile (gorge: the flag acts on library destinations only)", lir: "ChangeZoneAll obj=Target0 C=5 CS=from=Graveyard"},
	{card: "Expedition Raptor", form: "PutCounter Support$ 2 of a permanent (gorge putCounterSupport: Creature.+Other)", lir: "0 Support A=2 C=1 CS=P1P1"},
	{card: "Shoulder to Shoulder", form: "PutCounter Support$ 2 of a sorcery (gorge putCounterSupport: Creature)", lir: "0 Support A=2 CS=P1P1"},
	{card: "Irreverent Gremlin", form: "trigger ResolvedLimit$ 1 (gorge triggerTurnResolved: Tf2ResolvedLimit)", lir: "    resolvedlimit=1\n     0 IfPaid obj=You A=1 CS=Discard"},
}

func TestEffectStaticAndSupportForms(t *testing.T) { checkForms(t, effectstaticformsForms) }
