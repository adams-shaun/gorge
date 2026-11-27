package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Celebration (CR 700.14) counts the permanents that entered the battlefield
// under your control this turn, wherever the card is now: an Aura that entered,
// returned to its owner's hand and was cast again is still one of the two while
// it sits on the stack, so Armory Mice keeps its +0/+2.
func TestCelebrationCountsAPermanentRecastOntoTheStack(t *testing.T) {
	const scenario = `{"name":"celebration-recast-aura-on-stack","cr":["700.14"],"why":"a permanent that entered, left the battlefield and is now a spell on the stack still counts",
 "setup":{"p0":{"battlefield":["Armory Mice","Plains","Island"],"hand":["Wingspan Stride","Grizzly Bears"]},"p1":{"battlefield":["Mountain"]}},
 "steps":[
  {"op":"cast","seat":0,"card":"p0:Grizzly Bears","mana":"CG"},
  {"op":"pass","seat":0},{"op":"pass","seat":1},
  {"op":"cast","seat":0,"card":"p0:Wingspan Stride","mana":"U","targets":["p0:Armory Mice"]},
  {"op":"pass","seat":0},{"op":"pass","seat":1},
  {"op":"activate","seat":0,"card":"p0:Wingspan Stride","ability_index":0,"mana":"CCU"},
  {"op":"pass","seat":0},{"op":"pass","seat":1},
  {"op":"cast","seat":0,"card":"p0:Wingspan Stride","mana":"U","targets":["p0:Armory Mice"]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if p.Name == "Armory Mice" {
			if !strings.HasSuffix(p.PT, "/3") {
				t.Fatalf("Armory Mice is %s after the Aura was recast onto the stack; want x/3 (two nonland permanents entered)", p.PT)
			}
			return
		}
	}
	t.Fatal("Armory Mice not on the battlefield")
}
