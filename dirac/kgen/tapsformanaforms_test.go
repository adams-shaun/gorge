package kgen

import "testing"

// TestTapsForManaForms: the ordinary (non-static) Mode$ TapsForMana trigger
// that uses the stack (Manabarbs) lowers to a Taps line fired by a mana tap
// (kop.Tf5TapsForMana); the any-colour static extras (Buried in the Garden,
// Dawn's Reflection) need a colour ask at tap time the kernel cannot pose.
func TestTapsForManaForms(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Manabarbs", form: "ordinary TapsForMana trigger, body damages the tapper", lir: "trigger Taps"},
		{card: "Buried in the Garden", form: "static any-colour extra", lir: "TapExtra"},
		{card: "Dawn's Reflection", form: "static Combo Any extra", lir: "TapExtra"},
	})
}
