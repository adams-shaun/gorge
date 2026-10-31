package rules

import "testing"

// CR 601.2f: cost reductions apply to X's share of the total, so with RRRR and
// three Goblin Electromancers Blaze can announce X = 6: {6}{R} less {3} is four
// mana.
func TestXAnnouncementCountsCostReductions(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"blaze-three-electromancers","cr":["601.2f"],"why":"X = 6 is payable","setup":{"p0":{"battlefield":["Goblin Electromancer","Goblin Electromancer","Goblin Electromancer"],"hand":["Blaze"],"mana":"RRRR"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Blaze","mana":"RRRR","targets":["p1"],"answers":[{"kind":"choose","pick":["6"]}]},{"op":"resolve"}],"expect":[{"life":{"p1":14}}]}`)
}
