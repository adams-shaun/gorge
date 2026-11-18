package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Farm finding BLB `p0.exile` (xe:RAND:4:15:w9): gorge exiled [Forest], Forge
// [Might of the Meek], XMage [Scrapshooter]: three different cards. Alania's
// Pathmaker exiles "the top card of your library" (CR 701.13a: to exile an
// object, move it to the exile zone; CR 401.5/401.6 the top card is the first
// card of the library's order). With a scripted library the exiled card must
// be exactly library_top[0]. The three engines disagree only about which card
// that is, so their libraries differed before the trigger resolved: the window
// does not pin the library order (shuffle/RNG), which is a harness matter and
// not a gorge rules defect.
func TestFarmAlaniasPathmaker(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	scenario := `{
	  "name": "farm-alanias-pathmaker-exile-top",
	  "setup": {"p0": {"battlefield": ["Mountain","Mountain","Mountain","Mountain"], "hand": ["Alania's Pathmaker"],
	    "library_top": ["Forest","Island","Swamp","Plains"]}},
	  "steps": [
	    {"op": "activate", "seat": 0, "card": "p0:Mountain", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Mountain#2", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Mountain#3", "ability_index": 0},
	    {"op": "activate", "seat": 0, "card": "p0:Mountain#4", "ability_index": 0},
	    {"op": "cast", "seat": 0, "card": "p0:Alania's Pathmaker"},
	    {"op": "pass", "seat": 0},
	    {"op": "pass", "seat": 1},
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
	onBF := false
	for _, p := range last.Permanents {
		if p.Name == "Alania's Pathmaker" {
			onBF = true
		}
	}
	if !onBF {
		t.Fatalf("precondition: Alania's Pathmaker did not resolve")
	}
	p0 := last.Players[0]
	if !slices.Equal(p0.Exile, []string{"Forest"}) {
		t.Fatalf("p0 exile = %v, want exactly the scripted top card [Forest]", p0.Exile)
	}
	if len(p0.LibraryTop) < 3 || !slices.Equal(p0.LibraryTop[:3], []string{"Island", "Swamp", "Plains"}) {
		t.Fatalf("p0 library = %v, want the rest in order", p0.LibraryTop)
	}
}
