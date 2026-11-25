package kgen

import "testing"

// effectContainerTriggerModes: an Effect container whose Triggers$ line is a
// Mode$ the kernel's timed emblem listens to lowers as that emblem
// (lower_zone.go emblemTimedMode); a Mode$ gorge's Effect does not fire, or a
// duration the hook does not cover, stays refused by name. Every carrier of
// the Effect-container trigger-Mode ticket has a row: lowered, or refused.
var effectContainerTriggerModes = []formCase{
	{card: "Delif's Cone", form: "Effect AttackerUnblocked, captured target, ForgetOnMoved$", lir: "CreateEmblem"},
	{card: "Delif's Cube", form: "Effect AttackerUnblocked, PutCounter on the Effect's source", refused: "Defined$ \"EffectSource\""},
	{card: "Gaze of Pain", form: "Effect AttackerUnblocked, any creature you control", lir: "CreateEmblem"},
	{card: "Melee", form: "Effect AttackerUnblocked, Duration$ UntilEndOfCombat", refused: "Effect triggers without a permanent duration"},
	{card: "Jace, Cunning Castaway", form: "Effect DamageDoneOnce, Duration$ EndOfTurn spelled out (gorge's this-turn default)", lir: "CreateEmblem"},
	{card: "Vigorous Charge", form: "Effect DamageDealtOnce, CombatDamage$ True over a Targeted capture", lir: "CreateEmblem"},
	{card: "Paladin of Prahv", form: "Effect DamageDealtOnce without CombatDamage$ True (no lockstep cover: the bot never activates a forecast ability)", refused: "Effect triggers without a permanent duration"},
	{card: "Tamiyo, Field Researcher", form: "Effect DamageDealtOnce, UntilYourNextTurn capture of up to two targets", refused: "Effect triggers with RememberObjects$"},
	{card: "Don't Move", form: "Effect Taps, UntilYourNextTurn (the kernel's tap sites fire no emblem)", refused: "Effect triggers without a permanent duration"},
	{card: "Dynaheir, Invoker Adept", form: "Effect AbilityCast, OneOff$", refused: "Effect triggers without a permanent duration"},
	{card: "Magus Lucea Kane", form: "Effect SpellAbilityCast, OneOff$", refused: "Effect triggers without a permanent duration"},
	{card: "Forth Eorlingas!", form: "Effect DamageAll", refused: "Effect triggers without a permanent duration"},
	{card: "Giant Oyster", form: "Effect Phase, Untaps, ChangesZone, Duration$ UntilUntaps", refused: "Effect triggers without a permanent duration"},
	{card: "Silent Assassin", form: "Effect Phase EndCombat over a Targeted capture", refused: "Effect triggers without a permanent duration"},
	{card: "Mystic Reflection", form: "Effect ChangesZoneAll beside a replacement", refused: "Effect triggers without a permanent duration"},
	{card: "Storyweave", form: "Effect ChangesZoneAll beside a replacement", refused: "Effect triggers without a permanent duration"},
	{card: "Pure Intentions", form: "Effect DiscardedAll", refused: "Effect triggers without a permanent duration"},
	{card: "Season of the Bold", form: "Effect SpellCast, Duration$ UntilTheEndOfYourNextTurn", refused: "Effect triggers without a permanent duration"},
}

func TestEffectContainerTriggerModes(t *testing.T) { checkForms(t, effectContainerTriggerModes) }
