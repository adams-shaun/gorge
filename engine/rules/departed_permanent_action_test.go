package rules

import "testing"

// CR 701.50b: a conniver killed in response still connives by last known
// information. Its controller draws and discards; there is no counter to put.
func TestDepartedConniverStillDrawsAndDiscards(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"connive-departed","cr":["701.50b","701.50a"],"why":"conniver killed in response: controller still draws and discards","setup":{"p0":{"hand":["Red Room Recruit"],"library_top":["Island","Island"],"mana":"BBBBBB"},"p1":{"hand":["Lightning Strike"],"mana":"RR"}},"steps":[{"op":"cast","seat":0,"card":"p0:Red Room Recruit","mana":"BBB"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Lightning Strike","mana":"RR","targets":["p0:Red Room Recruit"]},{"op":"resolve"}],"expect":[{"card":"p0:Red Room Recruit","zone":"graveyard"},{"graveyard_size":{"p0":2}},{"hand_size":{"p0":0}}]}`)
}
