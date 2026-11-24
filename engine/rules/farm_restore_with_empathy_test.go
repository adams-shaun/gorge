package rules

import "testing"

// Farm finding FRA-s348-rand (xe:RAND:4:7:w15, FX|G, field stack): Danitha,
// Sword of Hope's "Whenever you cast ... a spell that targets a creature you
// control" fired in gorge on Restore with Empathy, whose only target is a
// permanent CARD IN THE GRAVEYARD. CR 109.2: a creature "you control" is a
// permanent, and CR 108.4/109.1: only objects on the battlefield (or stack) are
// controlled -- a card in a graveyard has an owner but no controller, so it is
// not "a creature you control" (CR 603.2: no trigger event). Forge and XMage
// both leave Restore with Empathy alone on the stack.
func TestFarmRestorewithEmpathy(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"restore-with-empathy-danitha","cr":["109.2","603.2"],"why":"graveyard creature card is not a creature you control: Danitha does not trigger","setup":{"p0":{"battlefield":["Danitha, Sword of Hope","Forest","Forest","Forest"],"hand":["Restore with Empathy"],"graveyard":["Grizzly Bears"],"library":["Forest","Forest"]},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Restore with Empathy","mana":"GGG","targets":["p0:Grizzly Bears"],"expect":[{"stack_size":1}]},{"op":"resolve"}],"expect":[{"card":"p0:Grizzly Bears","zone":"hand"},{"life":{"p0":24}}]}`)
}
