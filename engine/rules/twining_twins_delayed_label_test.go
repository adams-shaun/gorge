package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwiningTwinsDelayedTriggerLabelNamesAdventureFace pins CR 603.7d /
// 603.7: a delayed triggered ability's source is the source of the spell
// that created it. Swift Spiral's return is created by the Adventure face,
// so while it waits on the stack at the next end step (the card is by then
// back to its Twining Twins characteristics in exile, CR 715.4) its snapshot
// label names Swift Spiral, as the other engine names it, not the card's
// front face.
func TestTwiningTwinsDelayedTriggerLabelNamesAdventureFace(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sc := `{"name":"twins-delayed","setup":{"p0":{"hand":["Twining Twins"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Swift Spiral","mana":"WW","targets":["p1:Grizzly Bears"]},` +
		`{"op":"resolve","seat":0},{"op":"pass_to","seat":0,"step":"end","active":"p0"}]}`
	res, err := RunOracleScenarioJSON(reg, []byte(sc))
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	// Precondition: a snapshot has the delayed trigger (an ability) on the
	// stack; otherwise the label assertion below is vacuous.
	var abilities []OracleSnapStack
	for _, s := range res.Snapshots {
		for _, it := range s.Stack {
			if it.Kind == "ability" {
				abilities = append(abilities, it)
			}
		}
	}
	if len(abilities) == 0 {
		t.Fatal("no snapshot has the delayed trigger on the stack; scenario never reached it")
	}
	for _, it := range abilities {
		if want := "p0:Swift Spiral"; it.Source != want {
			t.Errorf("delayed trigger stack source = %q, want %q", it.Source, want)
		}
	}
}
