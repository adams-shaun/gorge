package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestBandingStaticAndActivatedGrantsLower: a static AddKeyword$ Banding
// (Dire Wolves, Banding Sliver) and an activated Pump KW$ Banding (Beast
// Walkers, Icatian Infantry, Soraya the Falconer) lower with no refusal.
func TestBandingStaticAndActivatedGrantsLower(t *testing.T) {
	names := []string{"Dire Wolves", "Banding Sliver", "Beast Walkers", "Icatian Infantry", "Soraya the Falconer"}
	for _, n := range names {
		bcs := lowerPoolBuilt(t, []string{n})
		if len(bcs) != 1 || bcs[0].name != n {
			t.Fatalf("%s: built %v", n, bcs)
		}
		if len(bcs[0].refusals) != 0 {
			t.Errorf("%s refused: %s", n, bcs[0].refusals[0].Reason)
		}
	}
}

// TestBandingGrantsBuildBandingRecords: the built bandingblock table carries
// each grant as an override record with Banding and no UntilEndCombat (a
// static's record lasts while the static applies, a Pump's until end of
// turn): Dire Wolves' and Banding Sliver's statics and Beast Walkers' and
// Icatian Infantry's Pumps, four in all.
func TestBandingGrantsBuildBandingRecords(t *testing.T) {
	set := cardsettest.Set(t, "bandingblock")
	tbl, _, _, err := set.Matchup(
		[]cardset.Entry{{Name: "Dire Wolves", Count: 4}, {Name: "Beast Walkers", Count: 4}, {Name: "Plains", Count: 16}, {Name: "Forest", Count: 16}},
		[]cardset.Entry{{Name: "Banding Sliver", Count: 4}, {Name: "Icatian Infantry", Count: 4}, {Name: "Plains", Count: 32}})
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for i, o := range tbl.Overrides {
		if !o.Banding {
			continue
		}
		n++
		if o.UntilEndCombat {
			t.Errorf("override %d grants Banding until end of combat: %+v", i, o)
		}
	}
	if n != 4 {
		t.Fatalf("%d override records grant Banding, want 4", n)
	}
}
