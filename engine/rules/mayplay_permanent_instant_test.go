package rules

// mayplay_permanent_instant_test.go pins CR 611.2a for an instant's Effect
// with an explicit Duration$ Permanent: Soul Partition's may-play grant (and
// its {2} surcharge) outlives the turn it was cast on, lasting for as long as
// the card stays exiled, and ends once the owner casts it.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestSoulPartitionMayPlayLaterTurn(t *testing.T) {
	offered := func(seat int, want bool) string {
		s := `{"op":"mana","seat":` + string(rune('0'+seat)) + `,"mana":"GGGGG","expect":[` +
			`{"card":"p1:Ambush Viper","zone":"exile"},` +
			`{"offered":{"seat":` + string(rune('0'+seat)) + `,"kind":"cast","card":"p1:Ambush Viper"},"want":`
		if want {
			return s + `true}]}`
		}
		return s + `false}]}`
	}
	const scenario = `{"name":"soul-partition-later-turn","cr":["611.2a","601.2"],"why":"inline",` +
		`"setup":{"p0":{"hand":["Soul Partition"]},"p1":{"battlefield":["Ambush Viper"]}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Soul Partition","mana":"CW","targets":["p1:Ambush Viper"]},` +
		`{"op":"resolve"},` +
		// Past this turn's cleanup, into P1's next turn.
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},` + "OFFER1," +
		`{"op":"pass_to","seat":0,"step":"main1","active":"p1"},` + "OFFER2," +
		// And again on a later turn (P0's, then P1's next).
		`{"op":"pass_to","seat":0,"step":"main1","active":"p0"},` + "OFFER3," +
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},` + "OFFER4," +
		`{"op":"cast","seat":1,"card":"p1:Ambush Viper","expect":[{"pool":{"p1":"G"}}]},` +
		`{"op":"resolve","expect":[{"card":"p1:Ambush Viper","zone":"battlefield"}]},` +
		// Cast: the permission is spent; the same card exiled again is new.
		`{"op":"move","seat":0,"card":"p1:Ambush Viper","to":"exile"},` +
		`{"op":"pass_to","seat":0,"step":"main1","active":"p1"},` +
		`{"op":"mana","seat":1,"mana":"GGGGG","expect":[` +
		`{"offered":{"seat":1,"kind":"cast","card":"p1:Ambush Viper"},"want":false}]}]}`
	// Offers: P1 holds it on both of its turns; P0 never does.
	s := strings.NewReplacer(
		"OFFER1", offered(1, true),
		"OFFER2", offered(1, true),
		"OFFER3", offered(0, false),
		"OFFER4", offered(1, true),
	).Replace(scenario)
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(s))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
