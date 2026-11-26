package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The ExiledWithSource readers outside a resolution-time mover
// (dirac/oracle/exiledwithsource_lockstep_test.go's pool): Ugin's Labyrinth
// counts the cards exiled with it in a mana ability's Amount$, Valakut
// Exploration counts them in an end-step Phase trigger's CheckSVar$, and
// Vault 13's chapter I exiles until the Saga leaves while chapter III returns
// two of them and sweeps the rest to the library (a hidden pick that
// remembers its cards, then a ChangeZoneAll that excludes them).
//
// The kernel reads each link against the matcher's own source (exwFlag reads
// ctx.src), the resolving entry's or the trigger's source; a match at each
// reader proves the read ran. The mutation drops the filter read's source
// (DebugExwStaticNoSrc, exwFlag) so a green Vault 13 sweep is not one where
// the link never mattered; Ugin's and Valakut's count arm (exwCount) is
// covered by the per-card matched reads instead.
const demoExiledwithsourcePool = "exiledwithsource"

var demoExiledwithsourceDecks = [2][]cardset.Entry{
	{
		{Name: "Ugin's Labyrinth", Count: 4}, {Name: "Valakut Exploration", Count: 2}, {Name: "Vault 13: Dweller's Journey", Count: 4},
		{Name: "Ulamog's Crusher", Count: 4}, {Name: "Savannah Lions", Count: 4}, {Name: "Suntail Hawk", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Plains", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Ugin's Labyrinth", Count: 4}, {Name: "Valakut Exploration", Count: 4}, {Name: "Vault 13: Dweller's Journey", Count: 2},
		{Name: "Ulamog's Crusher", Count: 4}, {Name: "Savannah Lions", Count: 3}, {Name: "Suntail Hawk", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 6},
	},
}

// demoExiledwithsourceGames plays seeds 1..hi and returns the clean games,
// the divergent ones, the link's counters, and the matched reads by reading
// card (the stats' table index mapped through the matchup's table).
func demoExiledwithsourceGames(t *testing.T, hi uint64, mode lockstepMode) (clean, divs int, st *dirac.ExwStats, byCard map[string]int) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoExiledwithsourcePool, demoExiledwithsourceDecks)
	tbl, _, _, err := set.Matchup(demoExiledwithsourceDecks[0], demoExiledwithsourceDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	st = &dirac.ExwStats{}
	dirac.DebugExwStats = st
	defer func() { dirac.DebugExwStats = nil }()
	clean, _, ds := lockstepSeedsMode(t, reg, set, demoExiledwithsourcePool, demoExiledwithsourceDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	byCard = map[string]int{}
	for i, n := range st.MatchBy {
		if n == 0 || i >= len(tbl.Cards) {
			continue
		}
		byCard[tbl.Cards[i].Name] += n
	}
	return clean, len(ds), st, byCard
}

// TestDemoExiledwithsourceLockstep: 64 seeds clean and each new reader
// matched its link at least once.
func TestDemoExiledwithsourceLockstep(t *testing.T) {
	clean, divs, st, byCard := demoExiledwithsourceGames(t, 64, lockstepFailEach)
	if divs > 0 {
		t.Errorf("%d divergent games", divs)
	}
	t.Logf("%d clean games, exw {Recorded:%d Reads:%d Matched:%d Rejected:%d AfterLeft:%d}, by card %v",
		clean, st.Recorded, st.Reads, st.Matched, st.Rejected, st.AfterLeft, byCard)
	for _, name := range []string{"Ugin's Labyrinth", "Valakut Exploration", "Vault 13: Dweller's Journey"} {
		if byCard[name] == 0 {
			t.Errorf("%s: no matched ExiledWithSource read", name)
		}
	}
	if st.Matched == 0 {
		t.Error("no ExiledWithSource read matched")
	}
}

// TestDemoExiledwithsourceMutation: the filter reader that drops its source
// (DebugExwStaticNoSrc, exwFlag) finds none of its cards, so Vault 13's sweep
// diverges. It does not touch the count arm Ugin's Labyrinth and Valakut
// Exploration use; their reads are proven by the per-card counts above.
func TestDemoExiledwithsourceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugExwStaticNoSrc = true
	defer func() { dirac.DebugExwStaticNoSrc = false }()
	_, divs, _, _ := demoExiledwithsourceGames(t, 64, lockstepQuiet)
	if divs == 0 {
		t.Fatal("DebugExwStaticNoSrc: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", divs)
}

// TestFourSeatDemoExiledwithsourceLockstep: the same readers in four-seat
// Commander pods, gorge leading.
func TestFourSeatDemoExiledwithsourceLockstep(t *testing.T) {
	a := pod4Deck(
		deckOf(4, "Ugin's Labyrinth"), deckOf(2, "Valakut Exploration"), deckOf(4, "Vault 13: Dweller's Journey"),
		deckOf(4, "Ulamog's Crusher"), deckOf(4, "Savannah Lions"), deckOf(4, "Suntail Hawk"),
		deckOf(4, "Grizzly Bears"), deckOf(2, "Hill Giant"),
		deckOf(4, "Plains"), deckOf(4, "Mountain"), deckOf(4, "Forest"))
	b := pod4Deck(
		deckOf(4, "Ugin's Labyrinth"), deckOf(4, "Valakut Exploration"), deckOf(2, "Vault 13: Dweller's Journey"),
		deckOf(4, "Ulamog's Crusher"), deckOf(3, "Savannah Lions"), deckOf(3, "Suntail Hawk"),
		deckOf(4, "Grizzly Bears"), deckOf(2, "Hill Giant"),
		deckOf(3, "Plains"), deckOf(5, "Mountain"), deckOf(6, "Forest"))
	if n := pod4FixtureLockstep(t, demoExiledwithsourcePool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
