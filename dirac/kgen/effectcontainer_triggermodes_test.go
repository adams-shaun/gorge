package kgen

import "testing"

// effectContainerTriggerModes: an Effect container whose Triggers$ line is a
// Mode$ the kernel's timed emblem listens to lowers as that emblem
// (lower_zone.go emblemTimedMode); a Mode$ gorge's Effect does not fire, or a
// duration the hook does not cover, stays refused by name.
var effectContainerTriggerModes = []formCase{
	{card: "Delif's Cone", form: "Effect AttackerUnblocked, captured target, ForgetOnMoved$", lir: "CreateEmblem"},
	{card: "Delif's Cube", form: "Effect AttackerUnblocked, PutCounter on the Effect's source", refused: "Defined$ \"EffectSource\""},
	{card: "Gaze of Pain", form: "Effect AttackerUnblocked, any creature you control", lir: "CreateEmblem"},
	{card: "Melee", form: "Effect AttackerUnblocked, Duration$ UntilEndOfCombat", refused: "Effect triggers without a permanent duration"},
}

func TestEffectContainerTriggerModes(t *testing.T) { checkForms(t, effectContainerTriggerModes) }
