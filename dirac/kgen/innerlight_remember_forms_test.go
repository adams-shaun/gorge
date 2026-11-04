package kgen

import "testing"

// innerlightRememberForms: an Effect's RememberObjects$ Targeted over its
// own ValidTgts$ creature is the emblem's capture, not a Remembered read of
// another ability's set (the census in lower_remembered.go sees no
// Remembered flag in the trigger), and the DamageDone body reads the damage
// dealt (TriggerAmount) since the kernel's emblem hook carries it.
var innerlightRememberForms = []formCase{
	{card: "The Inner Light", form: "chapter II Effect remembers its target; the trigger draws the damage dealt", lir: "emblem[0] trigger=2 statics=0 until=EOT remembers=Target0"},
	{card: "The Inner Light", form: "the emblem trigger's source filter is the type check only (the capture replaces IsRemembered)", lir: "trigger DamageDone F={types=0x2} F2={players=3}"},
	{card: "The Inner Light", form: "the draw reads the damage amount", lir: "0 Draw obj=You A=TriggerAmount"},
	{card: "Mistway Spy", form: "an Effect DamageDone trigger with no remembered object stays lowered", lir: "until=EOT"},
}

func TestRememberInnerLightForms(t *testing.T) { checkForms(t, innerlightRememberForms) }
