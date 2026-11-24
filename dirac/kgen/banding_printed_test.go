package kgen

import "testing"

// printedBandingCarriers print K:Banding (CR 702.22) with no other blocker:
// the printed head is CardDef.Banding (no KwMask bit is free), read by the
// kernel's hasBanding (dirac/kw_banding.go) for the CR 702.22c band ask and
// the CR 702.22j/k damage-division chooser, beside the granted overrides.
var printedBandingCarriers = []string{
	"Benalish Infantry", "Errand of Duty", "Icatian Phalanx", "Kjeldoran Escort", "Kjeldoran Knight",
	"Kjeldoran Phalanx", "Kjeldoran Skycaptain", "Kjeldoran Skyknight", "Kjeldoran Warrior", "Knights of Thorn",
	"Nalathni Dragon", "Noble Elephant", "Pikemen", "Shield Bearer", "Teremko Griffin",
	"Volunteer Reserves", "Wall of Shields", "War Elephant",
}

func TestPrintedBandingLowers(t *testing.T) {
	for _, name := range printedBandingCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}
