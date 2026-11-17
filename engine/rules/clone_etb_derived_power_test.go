package rules

import "testing"

// CR 613.4 with CR 707.2: Deceptive Frostkite's "a creature you control with
// power 4 or greater" reads layer-derived power. Silvercoat Lion (2/2) wearing
// Vectis Gloves (+2/+0) is a 4/2, a legal template for the as-enters copy,
// and the choice survives the replacement's recheck (the copy happens).
func TestDeceptiveFrostkiteCopiesEquippedCreature(t *testing.T) {
	t.Parallel()
	raw := `{"name":"frostkite-derived-power","cr":["613.4","707.2"],"why":"copy template power is the derived power","setup":{` +
		`"p0":{"battlefield":["Silvercoat Lion","Vectis Gloves"],"hand":["Deceptive Frostkite"]}},` +
		`"steps":[{"op":"activate","seat":0,"card":"p0:Vectis Gloves","mana":"CC","targets":["p0:Silvercoat Lion"],"ability":"Equip"},{"op":"resolve"},` +
		`{"op":"cast","seat":0,"card":"p0:Deceptive Frostkite","mana":"UU"},{"op":"resolve","answers":[{"kind":"choose","pick":["p0:Silvercoat Lion"]}]}],` +
		`"expect":[{"card":"p0:Deceptive Frostkite","zone":"battlefield","pt":"2/2"}]}`
	runETBCloneScenario(t, raw)
}
