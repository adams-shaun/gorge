package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Calamity, Galloping Inferno saddled only by a legendary creature has no
// nonlegendary creature to copy, so it makes no token; each of its two
// instructions still creates the delayed "sacrifice that token" triggered
// ability (CR 603.7), and both go on the stack at the end step and do nothing.
func TestCopyPermanentWithNothingToCopyStillCreatesItsDelayedTrigger(t *testing.T) {
	steps := `{"op":"activate","seat":0,"card":"p0:Calamity, Galloping Inferno","ability_index":0,"answers":[{"kind":"choose","pick":["p0:Isamaru, Hound of Konda"]}]},{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"attack","seat":0,"attackers":["p0:Calamity, Galloping Inferno"],"defender":"p1"}`
	for i := 0; i < 9; i++ {
		steps += `,{"op":"pass","seat":0},{"op":"pass","seat":1}`
	}
	scenario := `{"name":"calamity-no-copy","cr":["603.7"],"why":"the delayed triggers exist though no token does",
 "setup":{"p0":{"battlefield":["Calamity, Galloping Inferno","Isamaru, Hound of Konda"]},"p1":{"battlefield":["Mountain"]}},"steps":[` + steps + `]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	most := 0
	for _, s := range res.Snapshots {
		if s.Step != "end" {
			continue
		}
		n := 0
		for _, e := range s.Stack {
			if e.Kind == "ability" && strings.Contains(e.Source, "Calamity") {
				n++
			}
		}
		most = max(most, n)
	}
	if most != 2 {
		t.Fatalf("%d Calamity delayed triggers on the stack at the end step, want 2", most)
	}
}
