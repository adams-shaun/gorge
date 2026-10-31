package rules

import "testing"

// CR 608.2h: "the amount of mana spent to cast this creature" reads the creature
// as it last existed once it has left, so X keeps its value and the target stays
// legal.
func TestDepartedCreatureKeepsItsManaSpent(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"astelli-killed-spend-lki","cr":["608.2h"],"why":"X = mana spent on the Reclaimer that entered; dying in response leaves LKI 5","setup":{"p0":{"hand":["Astelli Reclaimer"],"graveyard":["Glorious Anthem"],"mana":"WWWWW"},"p1":{"hand":["Terror"],"mana":"BB"}},"steps":[{"op":"cast","seat":0,"card":"p0:Astelli Reclaimer","mana":"WWWWW"},{"op":"pass","seat":0},{"op":"pass","seat":1,"answers":[{"kind":"target","pick":["p0:Glorious Anthem"]}],"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Terror","mana":"BB","targets":["p0:Astelli Reclaimer"]},{"op":"resolve"}],"expect":[{"card":"p0:Astelli Reclaimer","zone":"graveyard"},{"card":"p0:Glorious Anthem","zone":"battlefield"}]}`)
}

// CR 608.2h: Converge's colours spent survive the creature leaving too.
func TestDepartedCreatureKeepsItsColoursSpent(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"epicure-bounced-converge","cr":["608.2h"],"why":"three colours spent; bounced in response X is still 3","setup":{"p0":{"hand":["Radiant Epicure"],"mana":"WUBBB"},"p1":{"hand":["Unsummon"],"mana":"U"}},"steps":[{"op":"cast","seat":0,"card":"p0:Radiant Epicure","mana":"WUBBB"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Unsummon","mana":"U","targets":["p0:Radiant Epicure"]},{"op":"resolve"}],"expect":[{"card":"p0:Radiant Epicure","zone":"hand"},{"life":{"p1":17,"p0":23}}]}`)
}
