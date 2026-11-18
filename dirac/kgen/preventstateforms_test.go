package kgen

import "testing"

// preventStateForms pins the scoped prevention lines that carry a rider
// (prevstate-01: Chant of Vitu-Ghazi gains the prevented life).
var preventStateForms = []formCase{
	{card: "Chant of Vitu-Ghazi", form: "scoped prevention gaining the prevented life", lir: "PreventDamageScope"},
	{card: "Suppressor Skyguard", form: "scoped combat prevention until end of combat", lir: "PreventDamageScope"},
	{card: "Awe Strike", form: "next damage from the remembered creature, gain that life", lir: "PreventNext"},
	{card: "Ria Ivor, Bane of Bladehold", form: "next combat damage to players this combat becomes tokens", lir: "PreventNext"},
}

// TestPreventStateForms pins the lowered and refused rider-prevention shapes.
func TestPreventStateForms(t *testing.T) { checkForms(t, preventStateForms) }
