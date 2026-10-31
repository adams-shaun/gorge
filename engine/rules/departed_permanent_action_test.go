package rules

import "testing"

// CR 701.50b: a conniver killed in response still connives by last known
// information. Its controller draws and discards; there is no counter to put.
func TestDepartedConniverStillDrawsAndDiscards(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"connive-departed","cr":["701.50b","701.50a"],"why":"conniver killed in response: controller still draws and discards","setup":{"p0":{"hand":["Red Room Recruit"],"library_top":["Island","Island"],"mana":"BBBBBB"},"p1":{"hand":["Lightning Strike"],"mana":"RR"}},"steps":[{"op":"cast","seat":0,"card":"p0:Red Room Recruit","mana":"BBB"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Lightning Strike","mana":"RR","targets":["p0:Red Room Recruit"]},{"op":"resolve"}],"expect":[{"card":"p0:Red Room Recruit","zone":"graveyard"},{"graveyard_size":{"p0":2}},{"hand_size":{"p0":0}}]}`)
}

// CR 701.44c: an explorer killed in response still explores by last known
// information, so a revealed land goes to its controller's hand.
func TestDepartedExplorerStillPutsLandInHand(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"explore-departed-land","cr":["701.44c","701.44a"],"why":"explorer killed in response: LKI; reveal; land to hand","setup":{"p0":{"hand":["Kinjalli's Dawnrunner"],"library_top":["Island"],"mana":"WWW"},"p1":{"hand":["Lightning Strike"],"mana":"RR"}},"steps":[{"op":"cast","seat":0,"card":"p0:Kinjalli's Dawnrunner","mana":"WWW"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Lightning Strike","mana":"RR","targets":["p0:Kinjalli's Dawnrunner"]},{"op":"resolve"}],"expect":[{"card":"p0:Kinjalli's Dawnrunner","zone":"graveyard"},{"hand_size":{"p0":1}}]}`)
}

// CR 701.44a,c: a departed explorer gets no counter, but its controller may
// still put a revealed nonland card into the graveyard, and it has explored.
func TestDepartedExplorerNonlandFiresExploresTrigger(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"explore-departed-nonland","cr":["701.44a","701.44b","701.44c"],"why":"nonland revealed for a departed explorer: card binned; Cave-Diver sees the explore","setup":{"p0":{"hand":["Kinjalli's Dawnrunner"],"battlefield":["Merfolk Cave-Diver"],"library_top":["Grizzly Bears"],"mana":"WWW"},"p1":{"hand":["Lightning Strike"],"mana":"RR"}},"steps":[{"op":"cast","seat":0,"card":"p0:Kinjalli's Dawnrunner","mana":"WWW"},{"op":"pass","seat":0},{"op":"pass","seat":1,"expect":[{"stack_size":1}]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Lightning Strike","mana":"RR","targets":["p0:Kinjalli's Dawnrunner"]},{"op":"resolve","answers":[{"kind":"any","pick":["Put it into your graveyard"]}]}],"expect":[{"card":"p0:Grizzly Bears","zone":"graveyard"},{"card":"p0:Merfolk Cave-Diver","pt":"3/4"}]}`)
}

// CR 701.44b: a permanent explores even when its library has no card to reveal.
func TestExploreWithEmptyLibraryStillExplores(t *testing.T) {
	t.Parallel()
	playRuleScenario(t, `{"name":"explore-empty-library","cr":["701.44b"],"why":"nothing to reveal; it still explores","setup":{"p0":{"hand":["Kinjalli's Dawnrunner"],"battlefield":["Merfolk Cave-Diver"],"mana":"WWW"},"p1":{}},"steps":[{"op":"cast","seat":0,"card":"p0:Kinjalli's Dawnrunner","mana":"WWW"},{"op":"resolve"},{"op":"resolve"},{"op":"resolve"}],"expect":[{"card":"p0:Merfolk Cave-Diver","pt":"3/4"}]}`)
}
