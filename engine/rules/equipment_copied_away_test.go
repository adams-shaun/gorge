package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// An Equipment that becomes a copy of a non-Equipment artifact is no
// Equipment while the copy lasts, so it becomes unattached (CR 301.5c,
// 704.5p): Quick-Draw Katana made a copy of Murmuring Volume by Shuri,
// Wakandan Inventor no longer sits on Donatello.
func TestEquipmentCopiedIntoAPlainArtifactBecomesUnattached(t *testing.T) {
	const scenario = `{"name":"equipment-copied-away","cr":["301.5c","704.5p"],"why":"an Equipment that stops being one detaches",
 "setup":{"p0":{"battlefield":["Island","Island","Quick-Draw Katana","Murmuring Volume","Shuri, Wakandan Inventor","Grizzly Bears"]},"p1":{"battlefield":["Mountain"]}},
 "steps":[
  {"op":"attach","seat":0,"card":"p0:Quick-Draw Katana","attached_to":"p0:Grizzly Bears"},
  {"op":"activate","seat":0,"card":"p0:Shuri, Wakandan Inventor","targets":["p0:Quick-Draw Katana","p0:Murmuring Volume"],"ability_index":0,"mana":"U"},
  {"op":"pass","seat":0},{"op":"pass","seat":1}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if p.Ref == "p0:Quick-Draw Katana" {
			if p.Name != "Murmuring Volume" {
				t.Fatalf("precondition: the Katana is %q, not a copy of Murmuring Volume", p.Name)
			}
			if p.AttachedTo != "" {
				t.Fatalf("the copied Katana is still attached to %s", p.AttachedTo)
			}
			return
		}
	}
	t.Fatal("Quick-Draw Katana not on the battlefield")
}
