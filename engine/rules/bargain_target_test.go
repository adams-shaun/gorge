package rules

import "testing"

// CR 702.166d: a target that matters only if the spell was bargained is chosen
// only then. Bargained Brave the Wilds animates its target and searches.
func TestBargainedSpellChoosesItsConditionalTarget(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"brave-the-wilds-bargained","cr":["702.166d"],"why":"Bargained: choose target land; it becomes 3/3 and search","setup":{"p0":{"hand":["Brave the Wilds"],"battlefield":["Forest","Bonesplitter"],"library":["Forest"],"mana":"G"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Brave the Wilds","mana":"G","cast_mode":"bargained","answers":[{"kind":"any","pick":["p0:Bonesplitter"]},{"kind":"target","pick":["p0:Forest"]}]},{"op":"resolve","answers":[{"kind":"any","pick":["Forest"]}]}],"expect":[{"card":"p0:Bonesplitter","zone":"graveyard"},{"card":"p0:Forest","pt":"3/3"},{"hand_size":{"p0":1}}]}`)
}

// CR 702.166d: unbargained, the spell is cast as if it had no such target, and
// the rest of it still happens.
func TestUnbargainedSpellStillDoesTheRest(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"brave-the-wilds-unbargained","cr":["702.166d"],"why":"Unbargained: no target; search only","setup":{"p0":{"hand":["Brave the Wilds"],"battlefield":["Forest"],"library":["Forest"],"mana":"G"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Brave the Wilds","mana":"G"},{"op":"resolve","answers":[{"kind":"any","pick":["Forest"]}]}],"expect":[{"hand_size":{"p0":1}}]}`)
}
