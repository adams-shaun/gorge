package rules

import (
	"strings"
	"testing"
)

// The cost-paid list selectors (cr-g261): `Defined$ Tapped`, `Defined$
// AttachedBy Returned` and `Defined$ Sacrificed` name the objects the cost was
// paid with, driven through real cards.

// TestPaidListSelectorsSwallowWhole: the Bears tapped for the additional cost
// get the +1/+1 counter, not the spell.
func TestPaidListSelectorsSwallowWhole(t *testing.T) {
	_, run := runFixtureScenario(t, `{"name":"swallow-whole","setup":{`+
		`"p0":{"battlefield":["Grizzly Bears"],"hand":["Swallow Whole"]},`+
		`"p1":{"battlefield":["Silvercoat Lion"],"tapped":["Silvercoat Lion"]}},`+
		`"steps":[{"op":"cast","seat":0,"card":"p0:Swallow Whole","mana":"W","targets":["p1:Silvercoat Lion"]},`+
		`{"op":"resolve","seat":0}],"expect":[`+
		`{"card":"p0:Grizzly Bears","pt":"3/3","counters":{"P1P1":1}},`+
		`{"count":{"seat":1,"zone":"exile","name":"Silvercoat Lion"},"eq":1},`+
		`{"count":{"seat":0,"zone":"graveyard","name":"Swallow Whole"},"eq":1}]}`)
	_ = run
}

// TestPaidListSelectorsKrasisIncubation: returning the Aura as the cost puts
// the two counters on the creature it enchanted (its LastBearer).
func TestPaidListSelectorsKrasisIncubation(t *testing.T) {
	_, run := runFixtureScenario(t, `{"name":"krasis-incubation","setup":{`+
		`"p0":{"hand":["Krasis Incubation"]},"p1":{"battlefield":["Grizzly Bears"]}},`+
		`"steps":[{"op":"cast","seat":0,"card":"p0:Krasis Incubation","mana":"CCGU","targets":["p1:Grizzly Bears"]},`+
		`{"op":"resolve","seat":0},`+
		`{"op":"activate","seat":0,"card":"p0:Krasis Incubation","ability_index":0,"mana":"CGU"},`+
		`{"op":"resolve","seat":0}],"expect":[`+
		`{"card":"p1:Grizzly Bears","pt":"4/4","counters":{"P1P1":2}},`+
		`{"count":{"seat":0,"zone":"hand","name":"Krasis Incubation"},"eq":1}]}`)
	_ = run
}

// TestPaidListSelectorsCleaverSkaab: sacrificing another Zombie as the cost
// makes two token copies of it.
func TestPaidListSelectorsCleaverSkaab(t *testing.T) {
	res, _ := runFixtureScenario(t, `{"name":"cleaver-skaab","setup":{`+
		`"p0":{"battlefield":["Cleaver Skaab","Walking Corpse"]}},`+
		`"steps":[{"op":"activate","seat":0,"card":"p0:Cleaver Skaab","ability_index":0,"mana":"CCC",`+
		`"answers":[{"kind":"choose","pick":["Walking Corpse"]}]},`+
		`{"op":"resolve","seat":0}],"expect":[`+
		`{"count":{"seat":0,"zone":"battlefield","name":"Walking Corpse"},"eq":2},`+
		`{"count":{"seat":0,"zone":"graveyard","name":"Walking Corpse"},"eq":1}]}`)
	if len(res.Fails) != 0 || !strings.Contains(strings.Join(res.Transcript, "\n"), "Walking Corpse") {
		t.Fatalf("transcript never names the sacrificed Zombie:\n%s", strings.Join(res.Transcript, "\n"))
	}
}
