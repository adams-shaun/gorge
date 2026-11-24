package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding SUP `p0.library_top` (xe:RAND:0:15:w5 and five siblings):
// Nissa's Pilgrimage finds exactly one basic Forest, so its closing Defined$
// Remembered leg (DBHand) has no card to move. Gorge removed the Forest and
// left the rest of the library in its setup order; XMage
// (SearchLibraryPutOntoBattlefieldTappedRestInHandEffect) shuffles. "Then
// shuffle" is owed once the search happened, whatever it found (CR 701.24a:
// randomize the library; CR 608.2c: follow the instructions in order). The
// main search and the battlefield fetch say Shuffle$ False and defer the
// shuffle to the Defined$ leg, which must still pay it when it moves nothing.
func TestFarmNissasPilgrimage(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	setupRest := []string{"Chatterstorm", "Eladamri's Call", "Arbor Colossus", "Plains", "Island",
		"Swamp", "Mountain", "Grizzly Bears", "Lightning Strike", "Giant Growth", "Llanowar Elves", "Shock"}
	scenario := `{
	  "name": "farm-nissas-pilgrimage-one-forest-shuffles",
	  "setup": {"p0": {"hand": ["Nissa's Pilgrimage"], "mana": "GGG",
	    "library_top": ["Chatterstorm","Eladamri's Call","Forest","Arbor Colossus","Plains","Island",
	      "Swamp","Mountain","Grizzly Bears","Lightning Strike","Giant Growth","Llanowar Elves","Shock"]}},
	  "steps": [
	    {"op": "cast", "seat": 0, "card": "p0:Nissa's Pilgrimage", "mana": "GGG"},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1, "answers": [{"kind": "choose", "pick": ["p0:Forest"]}, {"kind": "choose", "pick": ["p0:Forest"]}]}
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
	forestOnBF := false
	for _, p := range last.Permanents {
		if p.Name == "Forest" {
			forestOnBF = true
		}
	}
	if !forestOnBF {
		t.Fatalf("precondition: Nissa's Pilgrimage did not put the one Forest onto the battlefield")
	}
	lib := last.Players[0].LibraryTop
	// The snapshot shows the library's top five cards.
	if len(lib) != 5 || last.Players[0].LibraryCount < len(setupRest) {
		t.Fatalf("precondition: p0 library_top = %v (size %d), want the top 5 of %d cards", lib, last.Players[0].LibraryCount, len(setupRest))
	}
	if slices.Contains(lib, "Forest") {
		t.Fatalf("precondition: Forest still in library %v", lib)
	}
	if slices.Equal(lib, setupRest[:len(lib)]) {
		t.Fatalf("Nissa's Pilgrimage resolved without shuffling its controller's library: %v (setup order)", lib)
	}
}
