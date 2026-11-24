package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// oracleDieRollScenario is the Swarming Goblins ETB shape: a d20 whose 10-19
// band creates two Goblin tokens. The runner builds the board from the
// recorded natural roll, not its own seeded RNG, so a scenario carrying
// die_rolls reproduces the taped game (CR 706.2).
const oracleDieRollScenario = `{
  "name": "die-rolls-swarming-goblins",
  "setup": {
    "p0": {"hand": ["Swarming Goblins"]},
    "p1": {"battlefield": ["Grizzly Bears"]}
  },
  "die_rolls": [{"step": 1, "seat": 0, "sides": 20, "natural": 17, "result": 17}],
  "steps": [
    {"op": "cast", "seat": 0, "card": "p0:Swarming Goblins", "mana": "RRRRR"},
    {"op": "resolve", "seat": 0}
  ]
}`

func TestOracleScenarioForcesDieRolls(t *testing.T) {
	reg := testutil.CorpusRegistry(t)

	// Precondition: the same play without die_rolls takes the seeded d20's
	// other band, so the test's two-token assertion can only pass when the
	// recorded roll is forced.
	plain := strings.Replace(oracleDieRollScenario, `"die_rolls": [{"step": 1, "seat": 0, "sides": 20, "natural": 17, "result": 17}],`, "", 1)
	res, err := RunOracleScenarioJSON(reg, []byte(plain))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("unforced scenario failed: %v\n%s", res.Fails, joinLines(res.Transcript))
	}
	if got := oracleDieRollTokenCount(res, 0, "battlefield", "Goblin Token"); got != 1 {
		t.Fatalf("unforced scenario made %d Goblin tokens, want the seeded d20's 1-9 band (1)", got)
	}

	res, err = RunOracleScenarioJSON(reg, []byte(oracleDieRollScenario))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("forced scenario failed: %v\n%s", res.Fails, joinLines(res.Transcript))
	}
	if got := oracleDieRollTokenCount(res, 0, "battlefield", "Goblin Token"); got != 2 {
		t.Fatalf("forced scenario made %d Goblin tokens, want 2 (natural 17)", got)
	}
}

func TestOracleScenarioDieRollSidesMismatchFails(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	mismatch := strings.Replace(oracleDieRollScenario,
		`{"step": 1, "seat": 0, "sides": 20, "natural": 17, "result": 17}`,
		`{"step": 1, "seat": 0, "sides": 6, "natural": 3, "result": 3}`, 1)
	res, err := RunOracleScenarioJSON(reg, []byte(mismatch))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Fails) == 0 {
		t.Fatalf("mismatched die sides passed: no failure reported")
	}
	if !strings.Contains(strings.Join(res.Fails, "\n"), "harness") {
		t.Fatalf("mismatched die sides did not report a harness failure: %v", res.Fails)
	}
}

// oracleDieRollTokenCount counts the battlefield permanents with the given card name a
// seat controls, read from the scenario's final snapshot.
func oracleDieRollTokenCount(res OracleResult, seat int, zone, name string) int {
	if zone != "battlefield" || len(res.Snapshots) == 0 {
		return 0
	}
	snap := res.Snapshots[len(res.Snapshots)-1]
	n := 0
	for _, o := range snap.Permanents {
		if o.Controller == seat && strings.EqualFold(o.Name, name) {
			n++
		}
	}
	return n
}
