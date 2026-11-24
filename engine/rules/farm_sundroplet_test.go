package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `stack` (SUP-s119-supp xe:RAND:1:18:w10 and siblings):
// gorge [] vs XMage [ability:Sun Droplet@p0] at the beginning of an upkeep.
// Sun Droplet reads "At the beginning of each upkeep, you may remove a charge
// counter from Sun Droplet. If you do, you gain 1 life." Its trigger event has
// no intervening-if clause (CR 603.4 needs the "if" right after the trigger
// condition); the optional counter removal and "If you do" are resolution.
// CR 603.2: "Whenever a game event or game state matches a triggered
// ability's trigger event, that ability automatically triggers." So with no
// charge counter the trigger still goes on the stack (XMage's SunDroplet:
// BeginningOfUpkeepTriggeredAbility + DoIfCostPaid, no condition) and does
// nothing as it resolves; with a counter it resolves for 1 life.
func TestFarmSunDroplet(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	stackHasDroplet := func(snap OracleSnapshot) bool {
		for _, s := range snap.Stack {
			if s.Kind == "ability" && strings.Contains(s.Source, "Sun Droplet") {
				return true
			}
		}
		return false
	}
	t.Run("no counters", func(t *testing.T) {
		scenario := `{
		  "name": "farm-sun-droplet-no-counter",
		  "setup": {"p0": {"battlefield": ["Sun Droplet"]}},
		  "steps": [
		    {"op": "pass_to", "seat": 0, "step": "upkeep", "active": "p1"},
		    {"op": "resolve", "seat": 0, "expect": [{"life": {"p0": 20}}]}
		  ]
		}`
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
		}
		if len(res.Snapshots) < 2 {
			t.Fatalf("precondition: %d snapshots, want setup + pass_to + resolve", len(res.Snapshots))
		}
		last := res.Snapshots[len(res.Snapshots)-2]
		if last.Step != "upkeep" || last.Active != 1 {
			t.Fatalf("precondition: checkpoint at %s active p%d, want p1's upkeep", last.Step, last.Active)
		}
		for _, p := range last.Permanents {
			if p.Name == "Sun Droplet" && p.Counters["CHARGE"] != 0 {
				t.Fatalf("precondition: Sun Droplet has %d charge counters, want none", p.Counters["CHARGE"])
			}
		}
		if !stackHasDroplet(last) {
			t.Fatalf("stack = %+v, want the Sun Droplet upkeep trigger (CR 603.2: no intervening-if)", last.Stack)
		}
	})
	t.Run("one counter", func(t *testing.T) {
		scenario := `{
		  "name": "farm-sun-droplet-one-counter",
		  "setup": {"p0": {"battlefield": ["Sun Droplet"], "counters": {"Sun Droplet": {"CHARGE": 1}}}},
		  "steps": [
		    {"op": "pass_to", "seat": 0, "step": "upkeep", "active": "p1"},
		    {"op": "resolve", "seat": 0, "expect": [{"life": {"p0": 21}}]}
		  ]
		}`
		res, err := RunOracleScenarioJSON(reg, []byte(scenario))
		if err != nil {
			t.Fatalf("scenario JSON: %v", err)
		}
		if len(res.Fails) != 0 {
			t.Fatalf("%v\n%s", res.Fails, strings.Join(res.Transcript, "\n"))
		}
		if len(res.Snapshots) < 2 || !stackHasDroplet(res.Snapshots[len(res.Snapshots)-2]) {
			t.Fatalf("precondition: Sun Droplet trigger not on the stack at the upkeep")
		}
	})
}
