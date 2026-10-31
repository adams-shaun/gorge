package rules

import "testing"

// CR 702.187b, 400.7: a mayhem spell back in the graveyard is a new object that
// nobody discarded this turn, so mayhem is not offered again.
func TestMayhemNotOfferedAfterTheCardMoved(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"mayhem-recast-same-turn","cr":["702.187b","400.7"],"why":"Electro's Bolt cast for mayhem then back in the graveyard is a new object not discarded this turn","setup":{"p0":{"hand":["Tormenting Voice","Electro's Bolt"],"library":["Island","Island","Island"],"mana":"RRRRRR"},"p1":{"battlefield":["Grizzly Bears","Hill Giant"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Tormenting Voice","mana":"RR","answers":[{"kind":"choose","pick":["p0:Electro's Bolt"]}]},{"op":"resolve"},{"op":"cast","seat":0,"card":"p0:Electro's Bolt","mana":"RR","targets":["p1:Grizzly Bears"]},{"op":"resolve"}],"expect":[{"card":"p0:Electro's Bolt","zone":"graveyard"},{"card":"p1:Grizzly Bears","zone":"graveyard"},{"offered":{"seat":0,"kind":"cast","card":"p0:Electro's Bolt"},"want":false}]}`)
}
