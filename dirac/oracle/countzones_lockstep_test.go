package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The count-head lockstep: Backstreet Bruiser and Maester Seymour read
// Count$Valid ...$CardCounters.ALL (every counter kind summed); Frenzied
// Geistblaster, Machinist's Dismissal and Candlekeep Inspiration read
// multi-zone Count$Valid<Z1>,<Z2> heads (one count per zone over the same
// spec). Aerial Surveyor's Attacks trigger gates on CheckSVar$ over
// Land.DefenderCtrl, the defending player the attack bound.
var countZonesDecks = [2][]cardset.Entry{
	{{Name: "Backstreet Bruiser", Count: 4}, {Name: "Maester Seymour", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 14}, {Name: "Island", Count: 14}},
	{{Name: "Frenzied Geistblaster", Count: 4}, {Name: "Machinist's Dismissal", Count: 4}, {Name: "Candlekeep Inspiration", Count: 2},
		{Name: "Shock", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Island", Count: 11}, {Name: "Mountain", Count: 11}},
}

var countZonesSurveyorDecks = [2][]cardset.Entry{
	{{Name: "Aerial Surveyor", Count: 4}, {Name: "Grizzly Bears", Count: 10}, {Name: "Plains", Count: 13}, {Name: "Forest", Count: 13}},
	countZonesDecks[0],
}

func TestCountZonesLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "countzones", countZonesDecks, 200, "KCOUNTZONES_SEEDS")
}

func TestCountZonesSurveyorLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "countzones", countZonesSurveyorDecks, 200, "KCOUNTZONES_SEEDS")
}
