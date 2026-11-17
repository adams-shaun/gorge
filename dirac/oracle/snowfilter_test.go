package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Snow supertype filters: Skred counts the snow permanents you control,
// Chill to the Bone destroys nonsnow creatures, Priest of the Haunted Edge's
// ability targets snow lands you control and Abominable Treefolk counts snow
// permanents. kgen lowers the Snow head, the Snow property and nonSnow to
// PredWord7 kop.W7Snow / kop.W7NonSnow; the kernel reads the printed
// supertype, never while face down (dirac/filter_props.go). The fixture pool
// is dirac/cardset/pools/fixtures/snowfilter.

var snowFilterDecks = []fixtureDecks{{
	{
		{Name: "Skred", Count: 4}, {Name: "Chill to the Bone", Count: 4}, {Name: "Priest of the Haunted Edge", Count: 4},
		{Name: "Hill Giant", Count: 6}, {Name: "Snow-Covered Mountain", Count: 6}, {Name: "Snow-Covered Swamp", Count: 5},
		{Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Abominable Treefolk", Count: 4}, {Name: "Boreal Druid", Count: 4}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Snow-Covered Forest", Count: 6}, {Name: "Snow-Covered Island", Count: 5}, {Name: "Forest", Count: 7},
		{Name: "Island", Count: 8},
	},
}}

var snowFilterWatch = map[string]bool{"Skred": true, "Chill to the Bone": true, "Priest of the Haunted Edge": true, "Abominable Treefolk": true}

func TestSnowFilterLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := runFixtureLockstep(t, "snowfilter", snowFilterDecks, snowFilterWatch, 120, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.casts["Skred"] < 40 {
		t.Errorf("only %d Skred casts", run.casts["Skred"])
	}
	if run.casts["Chill to the Bone"] < 30 {
		t.Errorf("only %d Chill to the Bone casts", run.casts["Chill to the Bone"])
	}
}

// TestSnowFilterMutation: negating the Snow word's answers must diverge.
func TestSnowFilterMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSnowWordFlip = true
	defer func() { dirac.DebugSnowWordFlip = false }()
	run := runFixtureLockstep(t, "snowfilter", snowFilterDecks, snowFilterWatch, 120, false, nil)
	run.requireDivergence(t, "the Snow word is negated")
}
