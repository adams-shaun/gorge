package kgen

import "testing"

// A printed `S:Mode$ ReduceCost | EffectZone$ Battlefield` lowers: lowerStatic
// strips the bare default zone before the Mode body runs. The comma list
// (Battlefield,Command) is a Command-zone static and stays a refusal.
var reduceCostEffectZoneForms = []formCase{
	{card: "Artificer Class", form: "ReduceCost EffectZone$ Battlefield (first artifact spell)", lir: "static ReduceCost"},
	{card: "Hamza, Guardian of Arashin", form: "ReduceCost EffectZone$ Battlefield (creature spells with +1/+1 counter)", lir: "static ReduceCost"},
	{card: "Conduit of Ruin", form: "ReduceCost EffectZone$ Battlefield", lir: "static ReduceCost"},
	{card: "Starnheim Aspirant", form: "ReduceCost EffectZone$ Battlefield", lir: "static ReduceCost"},
	{card: "The Ur-Sphinx", form: "EffectZone$ Battlefield,Command stays refused (Command-zone static)", refused: `EffectZone "Battlefield,Command"`},
}

func TestReduceCostEffectZoneForms(t *testing.T) {
	checkForms(t, reduceCostEffectZoneForms)
}
