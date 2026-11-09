package kgen

import "testing"

// effectTrigParentTargetForms: an Effect link with no ValidTgts$ of its own
// that remembers Targeted captures the chain's one battlefield target (gorge
// remembers the union of the chain's target choices), so the emblem's
// DamageDone trigger fires only for the pumped creature.
var effectTrigParentTargetForms = []formCase{
	{card: "Captain Howler, Sea Scourge", form: "the Effect link remembers the pump's target slot", lir: "emblem[0] trigger=1 statics=0 until=EOT remembers=Target0"},
	{card: "Captain Howler, Sea Scourge", form: "the trigger's source filter is the type check only (the capture replaces IsRemembered)", lir: "trigger DamageDone F={types=0x2} F2={players=3}"},
}

func TestEffectTrigParentTarget(t *testing.T) { checkForms(t, effectTrigParentTargetForms) }

// effectDamageDoneNonPlayerForms: an Effect's DamageDone trigger whose
// ValidTarget$ is You fires only for combat damage to the emblem's owner
// (Tf2ToYou, the one-recipient flag the printed lines use).
var effectDamageDoneNonPlayerForms = []formCase{
	{card: "Right of Vengeance", form: "an until-end-of-turn Effect DamageDone trigger to You lowers", lir: "emblem[0] trigger=1 statics=0 until=EOT"},
	{card: "Right of Vengeance", form: "the trigger reads the damaged player as You (players=1)", lir: "trigger DamageDone F={types=0x2} F2={players=1}"},
}

func TestEffectDamageDoneNonPlayer(t *testing.T) { checkForms(t, effectDamageDoneNonPlayerForms) }
