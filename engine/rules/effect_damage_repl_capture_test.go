package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runEffectDamageReplScenario plays one inline XMage-port scenario and asserts
// it has no Fails and an empty log-only replay diff.
func runEffectDamageReplScenario(t *testing.T, sc string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) > 0 {
		t.Errorf("FAIL: %s\ntranscript:\n  %s", strings.Join(res.Fails, "\nFAIL: "), strings.Join(res.Transcript, "\n  "))
	}
	if res.ReplayDiff != "" {
		t.Errorf("log-only replay differs:\n%s", res.ReplayDiff)
	}
}

// TestEffectDamageReplShieldedPassage: Shielded Passage's bodiless
// `Event$ DamageDone | Prevent$ True | ValidTarget$ Card.IsRemembered` shield
// scopes by the Effect's own remembered set (the creature it targeted), CR 615.1a.
func TestEffectDamageReplShieldedPassage(t *testing.T) {
	t.Parallel()
	runEffectDamageReplScenario(t, `{"name":"passage","cr":["615.1a"],"why":"XMage AlmsBeastTest#testNoLifelinkAfterCombat","setup":{"p0":{"hand":["Shielded Passage"],"battlefield":["Rootwater Hunter","Plains"]},"p1":{"battlefield":["Alms Beast"]}},"steps":[{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},{"op":"attack","seat":1,"attackers":["p1:Alms Beast"],"defender":"p0"},{"op":"block","seat":0,"blocks":[["p0:Rootwater Hunter","p1:Alms Beast"]]},{"op":"pass","seat":1},{"op":"cast","seat":0,"card":"p0:Shielded Passage","mana":"W","targets":["p0:Rootwater Hunter"]},{"op":"pass_to","seat":0,"step":"main2","active":"p1"},{"op":"pass","seat":1},{"op":"activate","seat":0,"card":"p0:Rootwater Hunter","targets":["p1"],"ability_index":0},{"op":"pass_to","seat":0,"step":"end","active":"p1"},{"op":"resolve","seat":0}],"expect":[{"count":{"seat":0,"zone":"battlefield","name":"Rootwater Hunter"},"eq":1},{"life":{"p0":21}},{"life":{"p1":19}}]}`)
}

// TestEffectDamageReplForcefield: the Effect-created `ValidSource$
// Card.ChosenCard` shield reads the choice captured at creation, not the
// source's list after the chain's ClearChosenCard$ wiped it (CR 609.7).
func TestEffectDamageReplForcefield(t *testing.T) {
	t.Parallel()
	runEffectDamageReplScenario(t, `{"name":"forcefield","cr":["615.1a"],"why":"XMage ForcefieldTest#testForcefield","setup":{"p0":{"battlefield":["Bonebreaker Giant","Silvercoat Lion"]},"p1":{"battlefield":["Wastes","Wastes","Forcefield"]}},"steps":[{"op":"attack","seat":0,"attackers":["p0:Bonebreaker Giant","p0:Silvercoat Lion"],"defender":"p1"},{"op":"pass_to","seat":0,"step":"declare-blockers","active":"p0"},{"op":"pass","seat":0},{"op":"activate","seat":1,"card":"p1:Forcefield","mana":"C","ability_index":0},{"op":"activate","seat":1,"card":"p1:Forcefield","mana":"C","ability_index":0},{"op":"pass_to","seat":0,"step":"end","active":"p0","answers":[{"kind":"any","pick":["p0:Bonebreaker Giant"]},{"kind":"any","pick":["p0:Silvercoat Lion"]}]},{"op":"resolve","seat":0}],"expect":[{"life":{"p1":18}}]}`)
}
