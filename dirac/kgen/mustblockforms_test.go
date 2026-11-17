package kgen

import "testing"

// mustBlockForms: DB$ MustBlock (CR 509.1c) lowers to a this-turn
// requirement that the chosen creature blocks the named attacker if able.
var mustBlockForms = []formCase{
	{card: "Crashing Boars", form: "MustBlock Choices$ (defender chooses) blocks the source", lir: "MustBlockAttacker obj="},
	{card: "Avalanche Tusker", form: "MustBlock ValidTgts$ Creature.DefenderCtrl blocks the source", lir: "MustBlockAttacker obj="},
	{card: "Tower Above", form: "MustBlock on an Animate-granted trigger blocks the source", lir: "MustBlockAttacker obj="},
}

func TestMustBlockForms(t *testing.T) { checkForms(t, mustBlockForms) }
