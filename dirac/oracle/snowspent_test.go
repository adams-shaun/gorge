package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Count$CastTotalManaSpent Snow (CR 107.4h): Graven Lore, Berg Strider and
// Tundra Fumarole read the snow units their cast's payment took. The fixture
// pool is dirac/cardset/pools/fixtures/snowspent.

var snowSpentDecks = []fixtureDecks{{
	{
		{Name: "Graven Lore", Count: 4}, {Name: "Berg Strider", Count: 4}, {Name: "Tundra Fumarole", Count: 4},
		{Name: "Hill Giant", Count: 6}, {Name: "Snow-Covered Island", Count: 6}, {Name: "Snow-Covered Mountain", Count: 5},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 5},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Tundra Fumarole", Count: 2}, {Name: "Snow-Covered Mountain", Count: 5}, {Name: "Mountain", Count: 7},
		{Name: "Forest", Count: 10},
	},
}}

var snowSpentWatch = map[string]bool{"Graven Lore": true, "Berg Strider": true, "Tundra Fumarole": true}

func snowSpentRun(t *testing.T, report bool, mutate func(*kop.Table)) fixtureRun {
	t.Helper()
	return runFixtureLockstepPolicy(t, "snowspent", snowSpentDecks, 120, report, mutate, func(seed uint64, casts map[string]int) Policy {
		return snowWatchPolicy(seed, snowSpentWatch, casts)
	})
}

// TestSnowSpentLockstep: the kernel and gorge agree over the three carriers.
// The cast's spend is read as the spell resolves (Graven Lore's Scry, Tundra
// Fumarole's mana) or as its enters trigger queues (Berg Strider).
func TestSnowSpentLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	run := snowSpentRun(t, true, nil)
	t.Logf("%d games, %d divergences, %d stalls, casts %v", run.games, run.divs, run.stalls, run.casts)
	if run.divs != 0 || run.stalls != 0 {
		t.Errorf("%d divergences, %d stalls", run.divs, run.stalls)
	}
	for _, n := range []string{"Graven Lore", "Tundra Fumarole"} {
		if run.casts[n] < 30 {
			t.Errorf("%s was cast %d times in %d games, want at least 30", n, run.casts[n], run.games)
		}
	}
}

// TestSnowSpentMutation: recording the cast's mana value in place of its snow
// units must diverge.
func TestSnowSpentMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSnowSpentTotal = true
	defer func() { dirac.DebugSnowSpentTotal = false }()
	snowSpentRun(t, false, nil).requireDivergence(t, "the mana value recorded as the snow spent")
}
