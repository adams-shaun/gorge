package cardset_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestMostProminentMatchupKeepsCreatureTypes: Basalt Ravager counts the most
// prominent creature type among its controller's creatures, so a matchup
// holding it keeps the printed creature types on the creatures' masks (a
// creature type no filter names would otherwise be dropped and the count read
// 0); one without it drops them.
func TestMostProminentMatchupKeepsCreatureTypes(t *testing.T) {
	set := cardsettest.Synth(t, "tail-25")
	with, ids, _, err := set.Matchup([]cardset.Entry{{Name: "Basalt Ravager", Count: 1}, {Name: "Mountain", Count: 2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(with.Subtypes, "Giant") || !slices.Contains(with.Subtypes, "Wizard") {
		t.Fatalf("matchup with Basalt Ravager: Subtypes %v lack Giant or Wizard", with.Subtypes)
	}
	if !with.Cards[ids[0]].Subtypes.Any() {
		t.Fatalf("Basalt Ravager has an empty subtype mask")
	}

	without, _, _, err := set.Matchup([]cardset.Entry{{Name: "Mountain", Count: 2}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(without.Subtypes, "Giant") {
		t.Fatalf("control: Giant kept without a creature-type reader: %v", without.Subtypes)
	}
}
