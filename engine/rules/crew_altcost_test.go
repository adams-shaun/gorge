package rules

import "testing"

// Heart of Kiran: "You may remove a loyalty counter from a planeswalker you
// control rather than pay this Vehicle's crew cost" -- an AlternativeCost
// static scoped to the Vehicle's own crew ability (ValidSA$
// Activated.Crew+Vehicle, ValidCard$ Card.Self). CR 118.9 / CR 702.122.
// The option is its own "(alternate cost)" activation, offered only while a
// planeswalker of the activator's holds a loyalty counter, and tapping no
// creature leaves the crew marker unset.
func TestCrewAlternativeCostRemovesLoyalty(t *testing.T) {
	playRuleScenario(t, `{"name":"heart-of-kiran-crew","cr":["118.9","702.122"],"why":"inline",
	 "setup":{"p0":{"battlefield":["Heart of Kiran","Jace Beleren","Grizzly Bears"]}},
	 "steps":[
	  {"op":"pass_to","seat":0,"decision":"priority","expect":[
	    {"offered":{"seat":0,"kind":"ability","card":"p0:Heart of Kiran","label":"(alternate cost)"},"want":true}]},
	  {"op":"activate","seat":0,"card":"p0:Heart of Kiran","ability":"(alternate cost)"},
	  {"op":"resolve"}],
	 "expect":[{"card":"p0:Jace Beleren","counters":{"LOYALTY":2}},
	  {"card":"p0:Heart of Kiran","types":["Artifact","Creature","Vehicle"]},
	  {"card":"p0:Grizzly Bears","tapped":false}]}`)
}

func TestCrewAlternativeCostNeedsAPlaneswalker(t *testing.T) {
	playRuleScenario(t, `{"name":"heart-of-kiran-no-walker","cr":["118.9"],"why":"inline",
	 "setup":{"p0":{"battlefield":["Heart of Kiran","Grizzly Bears"]}},
	 "steps":[
	  {"op":"pass_to","seat":0,"decision":"priority","expect":[
	    {"offered":{"seat":0,"kind":"ability","card":"p0:Heart of Kiran","label":"(alternate cost)"},"want":false}]}]}`)
}
