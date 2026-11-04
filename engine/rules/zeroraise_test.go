package rules

// zeroraise_test.go pins Zero, Incorporeal Sensate ("Exile a nonland card
// from target opponent's hand. Until that card leaves exile, its owner may
// cast it.") beside the RaiseCost$ {2} the same Effect grants. The CostsMore
// static names no Activator$ (unlike Soul Partition's Activator$
// Opponent), so gorge's cost-actor match lets every actor pay the
// surcharge on a cast of the remembered card -- and the paired permission
// is MayPlayPlayer$ CardOwner, so the OWNER pays {2} more, opponent of
// Zero's controller or not. Nothing else on either player's cast list
// moves: a hand card the effect never remembered casts at its printed cost.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestZeroIncorporealSensateOwnerRaise(t *testing.T) {
	const scenario = `{"name":"zero-incorporeal-sensate-owner-raise","cr":["601.2","118.3a","611.2a","400.7"],"why":"inline",` +
		`"setup":{"p0":{"hand":["Zero, Incorporeal Sensate"]},"p1":{"hand":["Grizzly Bears","Giant Growth"]}},"steps":[` +
		// Zero enters; its trigger exiles a nonland card from p1's hand.
		`{"op":"cast","seat":0,"card":"p0:Zero, Incorporeal Sensate","mana":"CWB"},` +
		`{"op":"resolve","targets":["p1"],"answers":[{"kind":"choose","pick":["p1:Grizzly Bears"]}],` +
		`"expect":[{"card":"p1:Grizzly Bears","zone":"exile"}]},` +
		// The permission is the owner's: Zero's controller is offered nothing.
		`{"op":"mana","seat":0,"mana":"GGGGGGG","expect":[` +
		`{"offered":{"seat":0,"kind":"cast","card":"p1:Grizzly Bears"},"want":false}]},` +
		// A creature cast from exile is sorcery-speed: the offer appears only
		// in p1's own main phase, so the run walks there.
		`{"op":"pass_to","step":"main1","active":"p1"},` +
		// {3}{G}: three is not enough; four is exact and the pool empties.
		`{"op":"mana","seat":1,"mana":"GGG","expect":[` +
		`{"offered":{"seat":1,"kind":"cast","card":"p1:Grizzly Bears"},"want":false}]},` +
		`{"op":"mana","seat":1,"mana":"G","expect":[` +
		`{"offered":{"seat":1,"kind":"cast","card":"p1:Grizzly Bears"},"want":true}]},` +
		`{"op":"cast","seat":1,"card":"p1:Grizzly Bears","expect":[{"pool":{"p1":""}}]},` +
		`{"op":"resolve","expect":[{"card":"p1:Grizzly Bears","zone":"battlefield"}]},` +
		// The permission ended when the card left exile (ForgetOnMoved$ Exile,
		// CR 400.7), and a card the effect never remembered casts unraised.
		`{"op":"mana","seat":1,"mana":"G","expect":[` +
		`{"offered":{"seat":1,"kind":"cast","card":"p1:Giant Growth"},"want":true}]}]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
	}
}
