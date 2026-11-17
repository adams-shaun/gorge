package kgen

import "testing"

// ptSwitchForms: the hidden keyword "CARDNAME's power and toughness are
// switched" (CR 613.4d, layer 7d) on Pump lowers to a this-turn switch flag
// on the object (kernel P/T fold applies it after 7c).
var ptSwitchForms = []formCase{
	{card: "Twisted Image", form: "Pump KW$ HIDDEN switched + draw", lir: "SwitchPT"},
	{card: "Invert", form: "Pump up to two targets KW$ HIDDEN switched", lir: "SwitchPT"},
	{card: "About Face", form: "Pump target KW$ HIDDEN switched", lir: "SwitchPT"},
	{card: "Aeromoeba", form: "Pump Defined$ Self KW$ HIDDEN switched", lir: "SwitchPT"},
}

func TestPTSwitchForms(t *testing.T) { checkForms(t, ptSwitchForms) }
