package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Thorin, Mountain-king attaches Equipment to a creature you control, and
// "that creature deals damage equal to its power" through an ImmediateTrigger
// that remembers the creature the Equipment went onto. When that creature is
// Thorin himself, the object the reflexive ability remembers is also the one
// whose entering fired the spawning trigger; it is real memory all the same
// (CR 603.12), so the damage reads Thorin's power (3, plus 2 from Quick-Draw
// Katana on his controller's turn), not nothing.
func TestReflexiveRememberedMayBeTheSpawnersCapture(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct{ name, host, victim string }{
		{"onto Thorin", "p0:Thorin, Mountain-king", "p1:Undercity Eliminator"},
		{"onto another creature", "p0:Grizzly Bears", "p1:Undercity Eliminator"},
	} {
		raw := `{"name":"thorin","setup":{
		  "p0":{"battlefield":["Quick-Draw Katana","Grizzly Bears","Mountain","Mountain","Mountain","Mountain"],"hand":["Thorin, Mountain-king"]},
		  "p1":{"battlefield":["Undercity Eliminator"]}},
		 "steps":[{"op":"cast","seat":0,"card":"p0:Thorin, Mountain-king","mana":"RRRR"},{"op":"pass","seat":0},
		  {"op":"resolve","seat":0,"answers":[{"kind":"target","pick":["p0:Quick-Draw Katana"]},{"kind":"target","pick":["` + tc.host + `"]},{"kind":"target","pick":["` + tc.victim + `"]}]}]}`
		res, err := RunOracleScenarioJSON(reg, []byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%s: scenario failed: %v\n%s", tc.name, res.Fails, strings.Join(res.Transcript, "\n"))
		}
		last := res.Snapshots[len(res.Snapshots)-1]
		if _, alive := snapPerm(last, tc.victim); alive {
			t.Errorf("%s: %s survived: the creature the Equipment went onto dealt no damage", tc.name, tc.victim)
		}
	}
}
