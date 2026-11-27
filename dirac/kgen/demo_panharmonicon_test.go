package kgen

import "testing"

// demoPanharmoniconForms: the demo Panharmonicon statics this round lowers.
// Echoes of Eternity scopes its echo to the triggering object's own zone
// (a colorless spell on the stack or another colorless permanent on the
// battlefield); Panoptic Projektor names the turn-up event's causing object
// with ValidTurned$ and carries an Effect-delivered ReduceCost whose
// ForgetOnCast$ ends it on the next face-down creature spell.
var demoPanharmoniconForms = []formCase{
	{card: "Echoes of Eternity", form: "Panharmonicon ValidZone$ Battlefield,Stack", lir: "zones=72"},
	{card: "Panoptic Projektor", form: "Panharmonicon ValidMode$ TurnFaceUp + ValidTurned$", lir: "turned"},
	{card: "Panoptic Projektor", form: "Effect ReduceCost + ForgetOnCast$ (a face-down-spell discount emblem)", lir: "emblem[0] trigger=0 statics=1"},
}

func TestDemoPanharmoniconShapes(t *testing.T) { checkForms(t, demoPanharmoniconForms) }
