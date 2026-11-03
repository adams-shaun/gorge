package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// "Sacrifice them at the beginning of the next end step" on a spell that makes
// two tokens is one delayed triggered ability (CR 603.7), so Salt Road Skirmish
// puts one trigger on the stack at the end step, not one per Warrior, and
// that one trigger sacrifices both.
func TestAtEOTSacrificeOfSeveralTokensIsOneTrigger(t *testing.T) {
	steps := `{"op":"cast","seat":0,"card":"p0:Salt Road Skirmish","mana":"CCCB","targets":["p1:Grizzly Bears"]}`
	for i := 0; i < 3; i++ {
		steps += `,{"op":"pass","seat":0},{"op":"pass","seat":1}`
	}
	steps += `,{"op":"attack","seat":0,"attackers":["p0:token:Warrior Token","p0:token:Warrior Token#2"],"defender":"p1"}`
	for i := 0; i < 6; i++ {
		steps += `,{"op":"pass","seat":0},{"op":"pass","seat":1}`
	}
	scenario := `{"name":"salt-road-one-trigger","cr":["603.7"],"why":"one delayed trigger for both tokens",
 "setup":{"p0":{"battlefield":["Swamp"],"hand":["Salt Road Skirmish"]},"p1":{"battlefield":["Grizzly Bears"]}},
 "steps":[` + steps + `]}`
	res, err := RunOracleScenarioJSON(testutil.CorpusRegistry(t), []byte(scenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("scenario did not play cleanly: %v", res.Fails)
	}
	most, warriorsEver, warriorsLeft := 0, 0, 0
	for _, s := range res.Snapshots {
		n := 0
		for _, e := range s.Stack {
			if e.Kind == "ability" && strings.Contains(e.Source, "Salt Road Skirmish") {
				n++
			}
		}
		if n > most {
			most = n
		}
		w := 0
		for _, p := range s.Permanents {
			if strings.Contains(p.Types[0], "Creature") && p.Name == "token" || p.Name == "Warrior Token" {
				w++
			}
		}
		if w > warriorsEver {
			warriorsEver = w
		}
		warriorsLeft = w
	}
	if most != 1 {
		t.Fatalf("%d Skirmish triggers on the stack at once, want exactly 1", most)
	}
	if warriorsEver != 2 || warriorsLeft != 0 {
		t.Fatalf("warriors: peak %d, final %d; want 2 then 0 (the one trigger sacrifices both)", warriorsEver, warriorsLeft)
	}
}
