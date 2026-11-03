package rules

// count_valid_colors_layered_test.go pins that Count$Valid <spec>$Colors counts
// the colours permanents have now (CR 105.3, 613.1e), not their printed ones:
// Mondo Gecko turned white beside a blue creature draws for two colours.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestCountValidColorsReadsLayeredColours(t *testing.T) {
	const scenario = `{"name":"gecko-layered-colours","cr":["105.3","613.1e"],"why":"inline",` +
		`"setup":{"p0":{"battlefield":["Island","Mondo Gecko","Splash Lasher"],"hand":["Swamp"],` +
		`"library_top":["Forest","Mountain","Plains","Island"]},"p1":{}},"steps":[` +
		`{"op":"activate","seat":0,"card":"p0:Mondo Gecko","ability_index":0,"answers":[{"kind":"choose","pick":["p0:Swamp"]}]},` +
		`{"op":"resolve","answers":[{"kind":"choose","pick":["White"]}],"expect":[{"card":"p0:Mondo Gecko","colours":"W"}]},` +
		`{"op":"attack","seat":0,"attackers":["p0:Mondo Gecko"],"defender":"p1"},` +
		`{"op":"pass_to","step":"combat-damage"},` +
		`{"op":"resolve","expect":[{"hand_size":{"p0":2}}]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
