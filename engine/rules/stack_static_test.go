package rules

import "testing"

// CR 113.6: a permanent's static ability functions only on the battlefield, so a
// Quandrix still on the stack does not give the spell cast in response cascade.
func TestSpellOnTheStackGrantsNoStatic(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"quandrix-spell-on-stack-grants-cascade","cr":["113.6"],"why":"Quandrix and its own cascade trigger on the stack: Opt cast in response has no cascade","setup":{"p0":{"hand":["Quandrix, the Proof","Opt"],"library":["Grizzly Bears","Island","Island","Island"],"mana":"UUUUUUGGGGG"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Quandrix, the Proof","mana":"UUUUGGGG","expect":[{"stack_size":2}]},{"op":"cast","seat":0,"card":"p0:Opt","mana":"U"}],"expect":[{"stack_size":3}]}`)
}
