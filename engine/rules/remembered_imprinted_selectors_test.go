package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

func runRememberedImprintedScenario(t *testing.T, raw string) (fails, transcript []string) {
	t.Helper()
	sc, err := decodeOracleScenario([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	fails, transcript, _ = runOracleScenario(testutil.CorpusRegistry(t), sc)
	return fails, transcript
}

// TestRememberedImprintedSelectorsSupplantForm: "Return target creature to its
// owner's hand. You create a token that's a copy of that creature." The copy's
// `Defined$ DirectRemembered` names the returned creature's LKI, so p0 gets a
// Grizzly Bears token while the original sits in p1's hand.
func TestRememberedImprintedSelectorsSupplantForm(t *testing.T) {
	raw := `{"name":"supplant-form","why":"cr-g260","setup":{` +
		`"p0":{"hand":["Supplant Form"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Supplant Form","mana":"UUCCCC","answers":[` +
		`{"kind":"target","pick":["p1:Grizzly Bears"]}]},` +
		`{"op":"resolve","seat":0}],"expect":[` +
		`{"count":{"seat":0,"zone":"battlefield","name":"Grizzly Bears"},"eq":1},` +
		`{"count":{"seat":1,"zone":"battlefield","name":"Grizzly Bears"},"eq":0},` +
		`{"count":{"seat":1,"zone":"hand","name":"Grizzly Bears"},"eq":1}]}`
	fails, tr := runRememberedImprintedScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(tr, "\n"))
	}
}

// TestRememberedImprintedSelectorsMimicVat: a nontoken creature dies, the Vat
// imprints it, and `Defined$ Imprinted.ExiledWithSource` names that card for
// the copy: one Grizzly Bears token with haste.
func TestRememberedImprintedSelectorsMimicVat(t *testing.T) {
	raw := `{"name":"mimic-vat","why":"cr-g260","setup":{` +
		`"p0":{"battlefield":["Mimic Vat","Grizzly Bears"]}},` +
		`"steps":[{"op":"move","seat":0,"card":"p0:Grizzly Bears","to":"graveyard"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"trigger_optional","pick":["yes"]}],"expect":[` +
		`{"count":{"seat":0,"zone":"exile","name":"Grizzly Bears"},"eq":1},` +
		`{"count":{"seat":0,"zone":"battlefield","name":"Grizzly Bears"},"eq":0}]},` +
		`{"op":"activate","seat":0,"card":"p0:Mimic Vat","mana":"CCC","ability_index":0},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p0:Grizzly Bears"]}]}],"expect":[` +
		`{"count":{"seat":0,"zone":"battlefield","name":"Grizzly Bears"},"eq":1},` +
		`{"card":"p0:token:Grizzly Bears","zone":"battlefield","keywords":["Haste"]}]}`
	fails, tr := runRememberedImprintedScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(tr, "\n"))
	}
}

// TestRememberedImprintedSelectorsFiendHunter is the regression guard for the
// ChangeZone reader of DirectRemembered: Fiend Hunter exiles p1's creature,
// and the creature returns when Fiend Hunter leaves.
func TestRememberedImprintedSelectorsFiendHunter(t *testing.T) {
	raw := `{"name":"fiend-hunter","why":"cr-g260","setup":{` +
		`"p0":{"hand":["Fiend Hunter"]},"p1":{"battlefield":["Grizzly Bears"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Fiend Hunter","mana":"WWC"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"trigger_optional","pick":["yes"]},{"kind":"target","pick":["p1:Grizzly Bears"]}],"expect":[` +
		`{"count":{"seat":1,"zone":"exile","name":"Grizzly Bears"},"eq":1}]},` +
		`{"op":"move","seat":0,"card":"p0:Fiend Hunter","to":"graveyard"},` +
		`{"op":"resolve","seat":0}],"expect":[` +
		`{"count":{"seat":1,"zone":"battlefield","name":"Grizzly Bears"},"eq":1},` +
		`{"count":{"seat":1,"zone":"exile","name":"Grizzly Bears"},"eq":0}]}`
	fails, tr := runRememberedImprintedScenario(t, raw)
	if len(fails) != 0 {
		t.Fatalf("scenario failed: %v\n%s", fails, strings.Join(tr, "\n"))
	}
}
