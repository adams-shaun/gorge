package rules

// mayplay_cardowner_test.go pins the MayPlayPlayer$ CardOwner may-play
// beneficiary (Soul Partition: "Exile target nonland permanent. For as long
// as that card remains exiled, its owner may play it. A spell cast by an
// opponent this way costs {2} more to cast."): the Effect's grant benefits
// the exiled card's OWNER, not the Effect's controller, and its RaiseCost
// surcharge applies because that owner is an opponent of the controller.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestMayPlayCardOwnerSoulPartition(t *testing.T) {
	const scenario = `{"name":"soul-partition-card-owner","cr":["601.2","118.3a","611.2a"],"why":"inline",` +
		`"setup":{"p0":{"hand":["Soul Partition"]},"p1":{"battlefield":["Ambush Viper"]}},"steps":[` +
		`{"op":"cast","seat":0,"card":"p0:Soul Partition","mana":"CW","targets":["p1:Ambush Viper"]},` +
		`{"op":"resolve"},` +
		`{"op":"mana","seat":0,"mana":"GGGGG","expect":[` +
		`{"card":"p1:Ambush Viper","zone":"exile"},` +
		`{"offered":{"seat":0,"kind":"cast","card":"p1:Ambush Viper"},"want":false}]},` +
		`{"op":"pass","seat":0},` +
		`{"op":"mana","seat":1,"mana":"GGGGG","expect":[` +
		`{"offered":{"seat":1,"kind":"cast","card":"p1:Ambush Viper"},"want":true}]},` +
		`{"op":"cast","seat":1,"card":"p1:Ambush Viper","expect":[{"pool":{"p1":"G"}}]},` +
		`{"op":"resolve","expect":[{"card":"p1:Ambush Viper","zone":"battlefield"}]},` +
		// The permission ended when the card left exile (ForgetOnMoved$
		// Exile, CR 400.7): the same card exiled again is a new object.
		`{"op":"move","seat":0,"card":"p1:Ambush Viper","to":"exile"},` +
		`{"op":"pass","seat":0},` +
		`{"op":"mana","seat":1,"mana":"GGGGG","expect":[` +
		`{"card":"p1:Ambush Viper","zone":"exile"},` +
		`{"offered":{"seat":1,"kind":"cast","card":"p1:Ambush Viper"},"want":false}]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
	// The cast resolved under its owner's control (some snapshot holds the
	// Viper on p1's battlefield before the move back to exile).
	var resolved bool
	for _, s := range res.Snapshots {
		if v, ok := snapPerm(s, "p1:Ambush Viper"); ok && v.Controller == 1 && v.Owner == 1 {
			resolved = true
		}
	}
	if !resolved {
		t.Fatalf("Ambush Viper never resolved onto the battlefield under p1: %+v", res.Snapshots)
	}
}
