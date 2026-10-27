package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestCoatOfArmsChangelingReferent is the end-to-end pin for the
// sharesCreatureTypeWith referent read (CR 205.3m / 702.73a / 613.1d).
// Coat of Arms counts, per affected creature, the other creatures sharing a
// creature type with it, and Mistform Ultimus ("is every creature type") is
// the REFERENT. Its printed face is only Illusion, so the printed-face read
// counted no Grizzly Bears and gave Ultimus +0/+0; the derived layer-4 read
// makes every other creature a sharer: Ultimus 3/3 + 2 = 5/5, and each Bear
// and Lion 2/2 + 1 (Ultimus only) = 3/3.
func TestCoatOfArmsChangelingReferent(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const sc = `{"name":"coat","cr":["205.3m","702.73a"],"why":"Coat of Arms with Mistform Ultimus (XMage ChangelingTest#coatOfArmsTest shape)","setup":{"p0":{"battlefield":["Coat of Arms","Mistform Ultimus","Grizzly Bears","Silvercoat Lion"]}},"steps":[{"op":"resolve","seat":0}],"expect":[{"card":"p0:Mistform Ultimus","pt":"5/5"},{"card":"p0:Grizzly Bears","pt":"3/3"},{"card":"p0:Silvercoat Lion","pt":"3/3"}]}`
	res, err := RunOracleAuditScenarioJSON(reg, []byte(sc), true)
	if err != nil {
		t.Fatalf("scenario: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario fails: %v", res.Fails)
	}
	if res.ReplayDiff != "" {
		t.Fatalf("replay diff: %s", res.ReplayDiff)
	}
}
