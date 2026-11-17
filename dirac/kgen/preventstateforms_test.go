package kgen

import "testing"

// preventStateForms pins the scoped prevention lines that carry a rider
// (prevstate-01: Chant of Vitu-Ghazi gains the prevented life).
var preventStateForms = []formCase{
	{card: "Chant of Vitu-Ghazi", form: "scoped prevention gaining the prevented life", lir: "PreventDamageScope"},
}

// TestPreventStateForms pins the lowered and refused rider-prevention shapes.
func TestPreventStateForms(t *testing.T) { checkForms(t, preventStateForms) }
