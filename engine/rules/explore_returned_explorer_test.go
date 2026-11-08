package rules

import "testing"

// CR 400.7, 701.44c: Path of Discovery's trigger names a creature that was
// flickered in response. The old object still explores (reveals Hill Giant),
// but no counter goes on the new object. Forest in hand shows the new
// object's own trigger took the land, so the old trigger revealed Hill Giant.
func TestExploreTriggeredExplorerReturnedGetsNoCounter(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"explore-triggered-explorer-returned","cr":["400.7","701.44c"],"why":"the creature Path of Discovery's trigger names was flickered: it still explores, no counter on the new object","setup":{"p0":{"hand":["Grizzly Bears","Cloudshift"],"battlefield":["Path of Discovery"],"library_top":["Forest","Hill Giant"],"mana":"GGW"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"GG"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"cast","seat":0,"card":"p0:Cloudshift","mana":"W","targets":["p0:Grizzly Bears"]},{"op":"resolve","answers":[{"kind":"choose","pick":["top"]}]}],"expect":[{"card":"p0:Grizzly Bears","zone":"battlefield"},{"card":"p0:Forest","zone":"hand"},{"card":"p0:Hill Giant","zone":"library"},{"stack_size":0},{"card":"p0:Grizzly Bears","pt":"2/2"}]}`)
}

// CR 400.7, 701.44c: a self-explore ETB creature flickered in response to its
// own trigger still explores but the new object gets no counter.
func TestExploreSelfExplorerReturnedGetsNoCounter(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"explore-self-explorer-returned","cr":["400.7","701.44c"],"why":"Branchwalker flickered in response to its own explore trigger: no counter on the new object","setup":{"p0":{"hand":["Merfolk Branchwalker","Cloudshift"],"library_top":["Forest","Hill Giant"],"mana":"GGW"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Merfolk Branchwalker","mana":"GG"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"cast","seat":0,"card":"p0:Cloudshift","mana":"W","targets":["p0:Merfolk Branchwalker"]},{"op":"resolve","answers":[{"kind":"choose","pick":["top"]}]}],"expect":[{"card":"p0:Merfolk Branchwalker","zone":"battlefield"},{"card":"p0:Forest","zone":"hand"},{"stack_size":0},{"card":"p0:Merfolk Branchwalker","pt":"2/1"}]}`)
}
