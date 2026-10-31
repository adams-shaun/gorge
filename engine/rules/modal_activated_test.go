package rules

import "testing"

// CR 602.2b: a modal activated ability announces its mode and target as it is
// activated, so both are settled while it is on the stack.
func TestModalActivatedAbilityAnnouncesItsMode(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"phoenix-down-mode-at-activation","cr":["602.2b","601.2b"],"why":"mode and target announced at activation","setup":{"p0":{"battlefield":["Phoenix Down"],"graveyard":["Grizzly Bears"],"mana":"WW"},"p1":{"battlefield":["Hill Giant"]}},"steps":[{"op":"activate","seat":0,"card":"p0:Phoenix Down","mana":"WW","answers":[{"kind":"modes","pick":["Return target creature card"]}],"expect":[{"stack_size":1}]},{"op":"resolve"}],"expect":[{"card":"p0:Grizzly Bears","zone":"battlefield"}]}`)
}

// CR 608.2b: the mode's target was chosen at activation, so the ability fizzles
// if it is gone when the ability resolves.
func TestModalActivatedAbilityFizzlesWithoutItsTarget(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"phoenix-down-target-exiled","cr":["602.2b","608.2b"],"why":"the target is exiled in response, and the other Grizzly Bears was never the target","setup":{"p0":{"battlefield":["Phoenix Down"],"graveyard":["Grizzly Bears","Runeclaw Bear"],"mana":"WW"},"p1":{"hand":["Purify the Grave"],"mana":"W"}},"steps":[{"op":"activate","seat":0,"card":"p0:Phoenix Down","mana":"WW","answers":[{"kind":"modes","pick":["Return target creature card"]},{"kind":"target","pick":["p0:Grizzly Bears"]}],"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Purify the Grave","mana":"W","targets":["p0:Grizzly Bears"]},{"op":"resolve"}],"expect":[{"card":"p0:Grizzly Bears","zone":"exile"},{"card":"p0:Runeclaw Bear","zone":"graveyard"}]}`)
}

// CR 601.2c via 602.2b: with no mode that has a legal target the ability cannot
// be activated.
func TestModalActivatedAbilityNotOfferedWithoutALegalMode(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"phoenix-down-no-legal-mode","cr":["602.2b","601.2c"],"why":"no creature card in the graveyard and no Skeleton, Spirit or Zombie","setup":{"p0":{"battlefield":["Phoenix Down"],"mana":"WW"},"p1":{"battlefield":["Hill Giant"]}},"steps":[],"expect":[{"offered":{"seat":0,"kind":"activate","card":"p0:Phoenix Down"},"want":false}]}`)
}

// A mode's target may be the ability's own source (CR 115.5 excludes only a
// spell targeting itself).
func TestModalActivatedAbilityMayTargetItsSource(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"atalya-targets-itself","cr":["602.2b","115.5"],"why":"Atalya may prevent damage to herself","setup":{"p0":{"battlefield":["Atalya, Samite Master"],"mana":"WWW"},"p1":{}},"steps":[{"op":"activate","seat":0,"card":"p0:Atalya, Samite Master","mana":"WWW","answers":[{"kind":"choose","pick":["1"]},{"kind":"modes","pick":["Prevent the next"]},{"kind":"target","pick":["p0:Atalya, Samite Master"]}],"expect":[{"stack_size":1}]},{"op":"resolve"}],"expect":[{"card":"p0:Atalya, Samite Master","zone":"battlefield"}]}`)
}
