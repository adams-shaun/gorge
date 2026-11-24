package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

const feralLightningScenario = `{"name":"feral-lightning-one-trigger","cr":["603.7a","603.7b"],"why":"one delayed trigger for the group",
 "setup":{"p0":{"battlefield":["Mountain"],"hand":["Feral Lightning"]},"p1":{"battlefield":[]}},
 "steps":[{"op":"cast","seat":0,"card":"p0:Feral Lightning","mana":"CCCRRR"}`

// "Exile them at the beginning of the next end step" over the three tokens
// Feral Lightning creates is one delayed triggered ability (CR 603.7a/b): the
// end step puts one trigger on the stack and it exiles all three tokens.
func TestFeralLightningAtEOTIsOneTrigger(t *testing.T) {
	scenario := feralLightningScenario + passes(1) + `,{"op":"attack","seat":0,"attackers":[],"defender":"p1"}` + passes(5) + `]}`
	// Precondition: the three tokens are on the battlefield under the name
	// the stack-peak helper counts, so "none left" means they were exiled.
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	most := 0
	for _, s := range res.Snapshots {
		n := 0
		for _, p := range s.Permanents {
			if p.Name == "Elemental Token" {
				n++
			}
		}
		most = max(most, n)
	}
	if most != 3 {
		t.Fatalf("%d Elemental tokens on the battlefield at most, want 3", most)
	}
	peak, left := atEOTStackPeak(t, scenario, "Feral Lightning", "Elemental Token")
	if peak != 1 {
		t.Fatalf("%d Feral Lightning triggers on the stack at once, want 1", peak)
	}
	if left != 0 {
		t.Fatalf("%d Elemental tokens left, want 0 (the one trigger exiles all three)", left)
	}
}
