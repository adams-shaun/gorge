package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p1.exile` (xe:RAND:1:10:w14): gorge's Tawnos's Coffin
// activation resolved and exiled nothing, while Forge (tawnoss_coffin.txt,
// AB$ Effect -> SubAbility$ DBExile ChangeZoneAll Targeted.Self) and XMage
// (TawnossCoffinEffect.apply: moveCardsToExile) exile the target. "{3}, {T}:
// Exile target creature and all Auras attached to it." CR 701.13a: to exile an
// object, move it to the exile zone from wherever it is; CR 608.2c: the
// controller follows the instructions in the order written. After resolution
// the targeted creature is in its owner's exile and the Coffin stays tapped.
func TestFarmTawnossCoffin(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-tawnoss-coffin-exiles-target",
	  "setup": {"p0": {"battlefield": ["Tawnos's Coffin"]}, "p1": {"battlefield": ["Silvercoat Lion"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Tawnos's Coffin", "ability_index": 0, "mana": "CCC", "targets": ["p1:Silvercoat Lion"]},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1}
	  ]
	}`
	res, err := RunOracleScenarioJSON(reg, []byte(scenario))
	if err != nil {
		t.Fatalf("scenario JSON: %v", err)
	}
	if len(res.Fails) != 0 {
		t.Fatalf("%v", res.Fails)
	}
	last := res.Snapshots[len(res.Snapshots)-1]
	if len(last.Stack) != 0 {
		t.Fatalf("precondition: the activation did not resolve, stack = %v", last.Stack)
	}
	coffinTapped, coffinFound := false, false
	for _, p := range last.Permanents {
		if p.Name == "Silvercoat Lion" {
			t.Fatalf("Silvercoat Lion still on the battlefield after the Coffin resolved; p1 exile = %v", last.Players[1].Exile)
		}
		if p.Name == "Tawnos's Coffin" {
			coffinFound, coffinTapped = true, p.Tapped
		}
	}
	if !coffinFound || !coffinTapped {
		t.Fatalf("Tawnos's Coffin found=%v tapped=%v, want a tapped Coffin on the battlefield", coffinFound, coffinTapped)
	}
	if !slices.Contains(last.Players[1].Exile, "Silvercoat Lion") {
		t.Fatalf("p1 exile = %v, want Silvercoat Lion (CR 701.13a)", last.Players[1].Exile)
	}
}
