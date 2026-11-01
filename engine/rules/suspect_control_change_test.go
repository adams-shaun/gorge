package rules

import "testing"

// CR 701.60a: a creature stays suspected until it leaves the battlefield or an
// effect ends it. A change of controller does neither.
func TestSuspectedSurvivesControlChange(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"suspect-survives-control-change","cr":["701.60a"],"why":"suspected until it leaves or an effect ends it","setup":{"p0":{"hand":["Frantic Scapegoat"],"mana":"R"},"p1":{"hand":["Act of Treason"],"mana":"RRR"}},"steps":[{"op":"cast","seat":0,"card":"p0:Frantic Scapegoat","mana":"R"},{"op":"resolve"},{"op":"pass_to","seat":1,"step":"main1","active":"p1","answers":[{"kind":"any","pick":[]}]},{"op":"cast","seat":1,"card":"p1:Act of Treason","mana":"RRR","targets":["p0:Frantic Scapegoat"]},{"op":"resolve"}],"expect":[{"card":"p0:Frantic Scapegoat","keywords":["Menace"]}]}`)
}
