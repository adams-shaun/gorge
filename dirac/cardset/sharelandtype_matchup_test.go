package cardset_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestShareLandTypeMatchupKeepsBasicLandTypes: Myriad Landscape's second search
// pick compares the found lands' printed types in the kernel, so a matchup
// holding it keeps the basic land types on the lands' masks; one without it
// drops them (CR 701.23, 205.3i).
func TestShareLandTypeMatchupKeepsBasicLandTypes(t *testing.T) {
	set := cardsettest.Set(t, "cmd5")
	with, ids, _, err := set.Matchup([]cardset.Entry{{Name: "Myriad Landscape", Count: 1}, {Name: "Swamp", Count: 2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(with.Subtypes, "Swamp") {
		t.Fatalf("matchup with Myriad Landscape: Subtypes %v lack Swamp", with.Subtypes)
	}
	for _, id := range ids[1:] {
		if !with.Cards[id].Subtypes.Any() {
			t.Fatalf("Swamp card %d has an empty subtype mask", id)
		}
	}

	without, _, _, err := set.Matchup([]cardset.Entry{{Name: "Swamp", Count: 2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(without.Subtypes, "Swamp") {
		t.Fatalf("control: Swamp kept without the search flag: %v", without.Subtypes)
	}
}
