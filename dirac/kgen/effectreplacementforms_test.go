package kgen

import "testing"

// effectReplacementForms: Effect replacements whose Duration$ is tied to the source on the battlefield.
var effectReplacementForms = []formCase{
	{card: "Somnophore", form: "AsLongAsInPlay untap lock", lir: "SkipUntap"},
	{card: "Tidebinder Mage", form: "AsLongAsInPlay untap lock", lir: "SkipUntap"},
	{card: "Origin of Iron Man", form: "UntilHostLeavesPlay untap lock (Saga chapter)", lir: "SkipUntap"},
}

func TestEffectReplacementForms(t *testing.T) { checkForms(t, effectReplacementForms) }
