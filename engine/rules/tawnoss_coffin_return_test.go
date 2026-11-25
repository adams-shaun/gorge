package rules

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Tawnos's Coffin: "Exile target creature and all Auras attached to it. Note
// the number and kind of counters that were on that creature. When Tawnos's
// Coffin leaves the battlefield or becomes untapped, return that exiled card
// to the battlefield under its owner's control tapped with the noted number
// and kind of counters on it." The Effect's Triggers$ list is comma-separated
// (LeavesPlay,Untap); each body is a CR 603.7 delayed trigger. After p0's
// untap step untaps the Coffin, the Lion is back on p1's battlefield, tapped,
// with the two +1/+1 counters it had when it was exiled.
func TestTawnossCoffinReturnsOnUntap(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "tawnoss-coffin-returns-on-untap",
	  "setup": {"p0": {"battlefield": ["Tawnos's Coffin"]},
	            "p1": {"battlefield": ["Silvercoat Lion"], "counters": {"Silvercoat Lion": {"P1P1": 2}}}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Tawnos's Coffin", "ability_index": 0, "mana": "CCC", "targets": ["p1:Silvercoat Lion"]},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
	    {"op": "pass_to", "seat": 0, "step": "upkeep", "active": "p0"},
	    {"op": "pass_to", "seat": 0, "step": "main1", "active": "p0"}
	  ]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	var exiled bool
	for _, s := range res.Snapshots {
		if slices.Contains(s.Players[1].Exile, "Silvercoat Lion") {
			exiled = true
		}
	}
	if !exiled {
		t.Fatalf("precondition: the Lion was never exiled by the Coffin")
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	for _, p := range last.Permanents {
		if p.Name == "Tawnos's Coffin" && p.Tapped {
			t.Fatalf("precondition: the Coffin did not untap (step %s)", last.Step)
		}
	}
	for _, p := range last.Permanents {
		if p.Name != "Silvercoat Lion" {
			continue
		}
		if p.Controller != 1 || !p.Tapped || p.Counters["P1P1"] != 2 {
			t.Fatalf("returned Lion controller=%d tapped=%v counters=%v, want p1, tapped, P1P1=2", p.Controller, p.Tapped, p.Counters)
		}
		return
	}
	t.Fatalf("Silvercoat Lion did not return when the Coffin untapped; p1 exile = %v\n%s",
		last.Players[1].Exile, strings.Join(res.Transcript, "\n"))
}
