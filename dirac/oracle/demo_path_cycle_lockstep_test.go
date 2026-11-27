package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Path-of-the-X lockstep: the five demo Path sorceries. Four chain through
// Forge's DB$ BlankLine spacer, which gorge resolves as a silent no-op
// (effects/planechase.go effBlankLine); Path of the Schemer puts a picked
// graveyard creature onto the battlefield with StaticEffect$ Animate, which
// the kernel applies as the card enters (dirac/zone_pickoverride.go) like
// gorge's applyStaticEffect before the entry event (CR 603.6a). The pool is
// dirac/cardset/pools/fixtures/demopathcycle. The kernel's PickOverrideStats
// count proves the new pick rider fired, and Kappa Cannoneer, Master of
// Etherium and Thoughtcast make the added artifact type observable.
const demoPathCyclePool = "demopathcycle"

var demoPathCycleDecks = [2][]cardset.Entry{
	{
		{Name: "Path of the Animist", Count: 3}, {Name: "Path of the Schemer", Count: 4}, {Name: "Path of the Pyromancer", Count: 4},
		{Name: "Kappa Cannoneer", Count: 4}, {Name: "Master of Etherium", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Ornithopter", Count: 2}, {Name: "Sol Ring", Count: 2},
		{Name: "Swamp", Count: 4}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Path of the Enigma", Count: 4}, {Name: "Path of the Ghosthunter", Count: 4}, {Name: "Path of the Schemer", Count: 4},
		{Name: "Kappa Cannoneer", Count: 3}, {Name: "Thoughtcast", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Glory Seeker", Count: 3}, {Name: "Merfolk of the Pearl Trident", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 4},
	},
}

// demoPathCycleGames plays the seeds and returns the clean games, the
// divergent ones and the count of pick riders the kernel applied.
func demoPathCycleGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, st *dirac.PickOverrideStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoPathCyclePool, demoPathCycleDecks)
	st = &dirac.PickOverrideStats{}
	dirac.DebugPickOverrideStats = st
	defer func() { dirac.DebugPickOverrideStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoPathCyclePool, demoPathCycleDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, d, st
}

// TestDemoPathCycleLockstep: no divergence and no kernel stall over 64 seeds,
// and across them the Schemer's pick rider applied the Animate static.
func TestDemoPathCycleLockstep(t *testing.T) {
	clean, divs, st := demoPathCycleGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Applied == 0 {
		t.Error("no game applied a graveyard pick's StaticEffect$ override")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoPathCycleMutation: with the pick-override read disabled the picked
// creature enters without the Animate static, and some game diverges from
// gorge.
func TestDemoPathCycleMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPickOverrideOff = true
	defer func() { dirac.DebugPickOverrideOff = false }()
	_, divs, _ := demoPathCycleGames(t, 64, lockstepQuiet)
	if len(divs) == 0 {
		t.Fatal("DebugPickOverrideOff: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestFourSeatDemoPathCycleLockstep: four seats, gorge leading.
func TestFourSeatDemoPathCycleLockstep(t *testing.T) {
	a := pod4Deck(deckOf(3, "Path of the Animist"), deckOf(4, "Path of the Schemer"), deckOf(4, "Path of the Pyromancer"),
		deckOf(4, "Kappa Cannoneer"), deckOf(3, "Master of Etherium"), deckOf(4, "Grizzly Bears"),
		deckOf(2, "Ornithopter"), deckOf(2, "Sol Ring"), deckOf(4, "Swamp"), deckOf(3, "Forest"), deckOf(3, "Island"), deckOf(4, "Mountain"))
	b := pod4Deck(deckOf(4, "Path of the Enigma"), deckOf(4, "Path of the Ghosthunter"), deckOf(4, "Path of the Schemer"),
		deckOf(3, "Kappa Cannoneer"), deckOf(4, "Thoughtcast"), deckOf(4, "Grizzly Bears"),
		deckOf(3, "Glory Seeker"), deckOf(4, "Merfolk of the Pearl Trident"), deckOf(6, "Island"), deckOf(4, "Plains"))
	if n := pod4FixtureLockstep(t, demoPathCyclePool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
