package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// gorgeDayNightFixTime is gorge cr-daybound-nightbound-transform
// (0c28afe60), the later of the two day/night fixes (cr-daybound-nightbound
// 2e8eb2156 is 20261026101031).
const gorgeDayNightFixTime = "20261026102954"

// The day/night lockstep: Brutal Cathar and Graveyard
// Trespasser (daybound, their nightbound backs, Trespasser's ward—discard
// a card), against cheap spells that turn night back to day (Ornithopter,
// Shock) and Shock to target the ward. The expected answer at every
// decision is gorge's.
var kdaynightDecks = [2][]cardset.Entry{
	{
		{Name: "Brutal Cathar", Count: 8}, {Name: "Graveyard Trespasser", Count: 8}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Ornithopter", Count: 8}, {Name: "Shock", Count: 8},
		{Name: "Graveyard Trespasser", Count: 4}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 4},
	},
}

func TestDaynightLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeDayNightFixTime) {
		t.Skip("gorge before cr-daybound-nightbound-transform (0c28afe60)")
	}
	lockstepSet(t, "daynight", kdaynightDecks, 600, "KDAYNIGHT_SEEDS")
}

// TestDaynightLockstepMutation: with night never falling, some seed
// diverges.
func TestDaynightLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	if !gorgePinAtLeast(t, gorgeDayNightFixTime) {
		t.Skip("gorge before cr-daybound-nightbound-transform (0c28afe60)")
	}
	dirac.DebugNoNight = true
	defer func() { dirac.DebugNoNight = false }()
	if n := lockstepRun(t, "daynight", kdaynightDecks, 200, false, nil); n == 0 {
		t.Fatal("no seed diverged with night never falling")
	}
}
