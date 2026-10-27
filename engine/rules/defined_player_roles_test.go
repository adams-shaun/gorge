package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runRoleScenario(t *testing.T, raw string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	res, err := RunOracleScenarioJSON(reg, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}

// Gremlin Infestation: EnchantedController is the enchanted artifact's
// controller, not the Aura itself.
func TestDefinedPlayerRoleGremlinInfestation(t *testing.T) {
	runRoleScenario(t, `{"name":"gremlin-infestation","cr":["303.4"],"why":"EnchantedController names the artifact's controller",
 "setup":{"p0":{"hand":["Gremlin Infestation"],"battlefield":["Mountain","Mountain","Mountain","Mountain"]},"p1":{"battlefield":["Ornithopter"]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Gremlin Infestation","targets":["p1:Ornithopter"],"mana":"RRRR"},
  {"op":"resolve","seat":0},
  {"op":"pass_to","seat":0,"step":"end","active":"p0"},{"op":"resolve","seat":0}],
 "expect":[{"life":{"p0":20,"p1":18}},{"card":"p0:Gremlin Infestation","damage":0}]}`)
}

// Souls of the Faultless: AttackingPlayer is the attacker who dealt the damage.
func TestDefinedPlayerRoleSoulsOfTheFaultless(t *testing.T) {
	runRoleScenario(t, `{"name":"souls-faultless","cr":["508.1"],"why":"AttackingPlayer loses the life",
 "setup":{"p0":{"battlefield":["Souls of the Faultless"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},
  {"op":"attack","seat":1,"attackers":["p1:Grizzly Bears"],"defender":"p0"},
  {"op":"block","seat":0,"blocks":[["p0:Souls of the Faultless","p1:Grizzly Bears"]]},{"op":"pass","seat":1},
  {"op":"pass_to","seat":0,"step":"end","active":"p1"},{"op":"resolve","seat":0}],
 "expect":[{"life":{"p0":22,"p1":18}}]}`)
}
