package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Dominion Infiltration's copy token has changeling, so it is a Klingon; its
// sacrifice to Gowron counts although the token (an IsCopy object) has ceased
// to exist by resolution (CR 608.2h, 707.10h): draw a card and lose 1 life.
func TestGowronSacrificesAChangelingCopyToken(t *testing.T) {
	scenario := `{"name":"gowron-changeling-copy","cr":["608.2h","707.10h"],"why":"inline","setup":{"p0":{"battlefield":["Gowron, Eager for Battle","Mountain","Mountain","Mountain","Mountain"],` +
		`"hand":["Dominion Infiltration"],"library_top":["Island"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Dominion Infiltration","mana":"RRRR","answers":[{"kind":"choose","pick":["X = 1"]},{"kind":"target","pick":["p1:Grizzly Bears"]}]},{"op":"resolve"},` +
		`{"op":"activate","seat":0,"card":"p0:Gowron, Eager for Battle","answers":[{"kind":"choose","pick":["Grizzly Bears"]}]},{"op":"resolve"}],` +
		`"expect":[{"life":{"p0":19}}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
