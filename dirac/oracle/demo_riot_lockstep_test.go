package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted-Riot lockstep (CR 702.136a): Riot granted rather than printed.
// Spider-Punk prints Riot and grants it to other Spiders; Uncivil Unrest and
// Rhythm of the Wild grant it to the controller's nontoken creatures. The
// kernel reads each Continuous static's kop.Static.Riot beside CardDef.Riot
// (dirac/kw_riot.go); gorge's enteringRiot reads the derived keyword list, so
// the counter-or-haste election must be asked at the same decision. The pool
// is dirac/cardset/pools/fixtures/riotgrant. The kernel's Granted count proves
// a grant decided an election, so a green game is not one where no grant
// mattered. The decks hold one Uncivil Unrest; the multi-copy damage-doubling
// order is TestUncivilUnrestMultiCopyReplacementOrder's.
const demoRiotPool = "riotgrant"

var demoRiotDecks = [2][]cardset.Entry{
	{
		{Name: "Uncivil Unrest", Count: 1}, {Name: "Rhythm of the Wild", Count: 4},
		{Name: "Spider-Punk", Count: 2}, {Name: "Deadly Recluse", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Glory Seeker", Count: 4}, {Name: "Forest", Count: 9}, {Name: "Mountain", Count: 8},
	},
	{
		{Name: "Counterspell", Count: 4}, {Name: "Rhythm of the Wild", Count: 4},
		{Name: "Spider-Punk", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Glory Seeker", Count: 4}, {Name: "Deadly Recluse", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Forest", Count: 7}, {Name: "Island", Count: 7},
	},
}

// demoRiotGames plays the seeds and returns the clean games, the divergent
// ones and the granted elections the kernel posed.
func demoRiotGames(t *testing.T, hi uint64, mode lockstepMode) (clean, divs int, st *dirac.RiotStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoRiotPool, demoRiotDecks)
	st = &dirac.RiotStats{}
	dirac.DebugRiotStats = st
	defer func() { dirac.DebugRiotStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoRiotPool, demoRiotDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, len(d), st
}

// TestDemoRiotLockstep: no divergence and no kernel stall over 64 seeds, and
// across them a granted Riot posed an election.
func TestDemoRiotLockstep(t *testing.T) {
	clean, divs, st := demoRiotGames(t, 64, lockstepFailEach)
	if divs > 0 {
		t.Errorf("%d divergent games", divs)
	}
	if st.Granted == 0 {
		t.Error("no game posed an election through a granted Riot")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoRiotMutation: with the grant reader disabled, some game diverges
// from gorge.
func TestDemoRiotMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoGrantedRiot = true
	defer func() { dirac.DebugNoGrantedRiot = false }()
	_, divs, _ := demoRiotGames(t, 64, lockstepQuiet)
	if divs == 0 {
		t.Fatal("DebugNoGrantedRiot: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", divs)
}

// TestFourSeatDemoRiotLockstep: four seats, gorge leading.
func TestFourSeatDemoRiotLockstep(t *testing.T) {
	a := pod4Deck(deckOf(1, "Uncivil Unrest"), deckOf(4, "Rhythm of the Wild"), deckOf(2, "Spider-Punk"),
		deckOf(4, "Deadly Recluse"), deckOf(4, "Giant Spider"), deckOf(4, "Grizzly Bears"),
		deckOf(4, "Glory Seeker"), deckOf(9, "Forest"), deckOf(8, "Mountain"))
	b := pod4Deck(deckOf(4, "Counterspell"), deckOf(4, "Rhythm of the Wild"), deckOf(2, "Spider-Punk"),
		deckOf(4, "Grizzly Bears"), deckOf(4, "Glory Seeker"), deckOf(4, "Deadly Recluse"),
		deckOf(4, "Giant Spider"), deckOf(7, "Forest"), deckOf(7, "Island"))
	c := pod4Deck(deckOf(4, "Rhythm of the Wild"), deckOf(4, "Spider-Punk"), deckOf(4, "Deadly Recluse"),
		deckOf(4, "Giant Spider"), deckOf(4, "Grizzly Bears"), deckOf(4, "Glory Seeker"),
		deckOf(8, "Forest"), deckOf(8, "Mountain"))
	d := pod4Deck(deckOf(4, "Rhythm of the Wild"), deckOf(4, "Spider-Punk"), deckOf(4, "Deadly Recluse"),
		deckOf(4, "Giant Spider"), deckOf(4, "Grizzly Bears"), deckOf(4, "Glory Seeker"),
		deckOf(8, "Forest"), deckOf(8, "Mountain"))
	if n := pod4FixtureLockstep(t, demoRiotPool, [4][]string{a, b, c, d}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
