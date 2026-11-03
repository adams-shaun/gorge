package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "This ability triggers only once each turn" bounds the additional times a
// Panharmonicon-style static makes an ability trigger (CR 603.2d): Elvish
// Archivist's enchantment draw puts one trigger on the stack when Virtue of
// Knowledge enters, not the pair the static would otherwise make.
func TestOncePerTurnTriggerIsNotDoubledByAnExtraTriggerStatic(t *testing.T) {
	const scenario = `{"name":"archivist-virtue","cr":["603.2d"],"why":"a once-per-turn ability triggers once",
 "setup":{"p0":{"battlefield":["Elvish Archivist"],"hand":["Virtue of Knowledge"]},"p1":{"battlefield":["Mountain"]}},
 "steps":[
  {"op":"cast","seat":0,"card":"p0:Virtue of Knowledge","mana":"CCCCU"},
  {"op":"pass","seat":0},{"op":"pass","seat":1}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	n := 0
	for _, e := range last.Stack {
		if e.Kind == "ability" && strings.Contains(e.Source, "Elvish Archivist") {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d Elvish Archivist triggers on the stack, want 1", n)
	}
}
