package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runSplitDamageScenario runs an XMage-port scenario in fixture mode and
// fails on any scenario failure or log-replay difference.
func runSplitDamageScenario(t *testing.T, sc string) OracleAuditResult {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil {
		t.Fatalf("scenario: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario fails: %v\n%s", res.Fails, res.Transcript)
	}
	if res.ReplayDiff != "" {
		t.Fatalf("log-only replay differs:\n%s", res.ReplayDiff)
	}
	return res
}

// TestReplaceSplitDamageWardOfPiety: two activations each redirect the next 1
// damage to p1; Bolt on the Lion deals 1 to the Lion and 2 to p1 (CR 614.9).
func TestReplaceSplitDamageWardOfPiety(t *testing.T) {
	t.Parallel()
	runSplitDamageScenario(t, `{"name":"ward","cr":["614.9"],"why":"XMage WardOfPietyTest#testNonCombatDamageToPlayer","setup":{"p0":{"hand":["Ward of Piety"],"battlefield":["Silvercoat Lion","Plains","Plains","Plains","Plains","Plains","Plains"]},"p1":{"hand":["Lightning Bolt"],"battlefield":["Mountain"]}},"steps":[{"op":"resolve","seat":0},{"op":"cast","seat":0,"card":"p0:Ward of Piety","mana":"CW","targets":["p0:Silvercoat Lion"]},{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0"},{"op":"activate","seat":0,"card":"p0:Ward of Piety","mana":"CW","targets":["p1"],"ability_index":0},{"op":"activate","seat":0,"card":"p0:Ward of Piety","mana":"CW","targets":["p1"],"ability_index":0},{"op":"pass_to","seat":0,"step":"main2","active":"p0"},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Lightning Bolt","mana":"R","targets":["p0:Silvercoat Lion"]},{"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Silvercoat Lion"},"eq":1},{"life":{"p0":20}},{"life":{"p1":18}}]}`)
}

// TestReplaceSplitDamageShamanEnKor: two 1-damage shields redirect 1+1 of a
// Bolt to the Lion, the Shaman takes the remaining 1 and lives.
func TestReplaceSplitDamageShamanEnKor(t *testing.T) {
	t.Parallel()
	runSplitDamageScenario(t, `{"name":"enkor","cr":["614.9"],"why":"XMage ShamenEnKorTest#testFirstAbilityNonCombatDamage","setup":{"p0":{"hand":["Lightning Bolt"],"battlefield":["Mountain"]},"p1":{"battlefield":["Shaman en-Kor","Silvercoat Lion"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Lightning Bolt","mana":"R","targets":["p1:Shaman en-Kor"]},{"op":"pass","seat":0},{"op":"activate","seat":1,"card":"p1:Shaman en-Kor","targets":["p1:Silvercoat Lion"],"ability_index":0},{"op":"activate","seat":1,"card":"p1:Shaman en-Kor","targets":["p1:Silvercoat Lion"],"ability_index":0},{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0"},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":1,"zone":"battlefield","name":"Shaman en-Kor"},"eq":1},{"count":{"seat":1,"zone":"graveyard","name":"Silvercoat Lion"},"eq":1}]}`)
}

// TestReplaceSplitDamageHarmsWay: Harm's Way's target is announced at cast
// (CR 601.2c) and redirects 2 of Bolt's 3 to p0.
func TestReplaceSplitDamageHarmsWay(t *testing.T) {
	t.Parallel()
	runSplitDamageScenario(t, `{"name":"harms","cr":["601.2c","614.9"],"why":"XMage HarmsWayRedirectDamageTest#testRedirectTwoDamage","setup":{"p0":{"hand":["Lightning Bolt"],"battlefield":["Mountain"]},"p1":{"hand":["Harm's Way"],"battlefield":["Plains"]}},"steps":[{"op":"cast","seat":0,"card":"p0:Lightning Bolt","mana":"R","targets":["p1"]},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Harm's Way","mana":"W","targets":["p0"]},{"op":"pass_to","seat":0,"step":"end","active":"p0","answers":[{"kind":"any","pick":["p0:Lightning Bolt"]}]},{"op":"resolve","seat":0}],"expect":[{"life":{"p0":18}},{"life":{"p1":19}}]}`)
}

// TestReplaceSplitDamageRegistered pins the census token: the primitive is
// supported and Ward of Piety no longer reports it unsupported.
func TestReplaceSplitDamageRegistered(t *testing.T) {
	t.Parallel()
	if !effects.Supported()["api:ReplaceSplitDamage"] {
		t.Fatal(`effects.Supported() is missing "api:ReplaceSplitDamage"`)
	}
	reg := sharedCorpus(t)
	ward := mustCorpusCard(t, reg, "Ward of Piety")
	if !slices.Contains(ward.Primitives(), "api:ReplaceSplitDamage") {
		t.Fatalf("precondition failed: Ward of Piety primitives %v lack api:ReplaceSplitDamage", ward.Primitives())
	}
	if missing := reg.Unsupported(ward, effects.Supported()); slices.Contains(missing, "api:ReplaceSplitDamage") {
		t.Fatalf("Ward of Piety still reports api:ReplaceSplitDamage unsupported: %v", missing)
	}
}

// TestReplaceSplitDamageShieldDepletes: a one-use Ward of Piety shield spends
// across events (CR 615.7 by analogy). A Bolt and a Shock at the enchanted Craw Wurm
// redirect 1 damage to p1 once, not twice: p1 ends at 19 (an undepleted shield
// would redirect on both spells, leaving p1 at 18).
func TestReplaceSplitDamageShieldDepletes(t *testing.T) {
	t.Parallel()
	runSplitDamageScenario(t, `{"name":"deplete","cr":["614.9"],"why":"a redirect shield is spent by what it redirects","setup":{"p0":{"hand":["Ward of Piety"],"battlefield":["Craw Wurm","Plains","Plains","Plains","Plains"]},"p1":{"hand":["Lightning Bolt","Shock"],"battlefield":["Mountain","Mountain"]}},"steps":[{"op":"resolve","seat":0},{"op":"cast","seat":0,"card":"p0:Ward of Piety","mana":"CW","targets":["p0:Craw Wurm"]},{"op":"pass_to","seat":0,"step":"begin-combat","active":"p0"},{"op":"activate","seat":0,"card":"p0:Ward of Piety","mana":"CW","targets":["p1"],"ability_index":0},{"op":"pass_to","seat":0,"step":"main2","active":"p0"},{"op":"pass","seat":0},{"op":"cast","seat":1,"card":"p1:Lightning Bolt","mana":"R","targets":["p0:Craw Wurm"]},{"op":"cast","seat":1,"card":"p1:Shock","mana":"R","targets":["p0:Craw Wurm"]},{"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}],"expect":[{"life":{"p0":20}},{"life":{"p1":19}}]}`)
}
