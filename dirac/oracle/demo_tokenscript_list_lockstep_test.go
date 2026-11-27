package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The comma-listed TokenScript$ lockstep: Academy Manufactor's CreateToken
// rewriter (a Clue, Food or Treasure token its controller would create is one
// of each instead; gorge applyTokenReplacementToPlan's ReplaceToken arm,
// dirac/repl_toklistrewrite.go) and Overencumbered (its enters trigger has the
// enchanted opponent create a Clue, a Food and a Junk token; Defined$
// EnchantedPlayer, kop.RefEnchantedPlayer; the Phase trigger over
// Opponent.EnchantedBy, kop.WhoseEnchantedOpp; the "creatures can't attack"
// emblem that ends with the combat, kop.Emblem.UntilEndCombat). Clue, Food and
// Treasure makers feed the rewriter and Dragon Fodder's Goblins must stay
// untouched. The pool is dirac/cardset/pools/fixtures/demotokenscriptlist.
const demoTokenscriptListPool = "demotokenscriptlist"

var demoTokenscriptListDecks = [2][]cardset.Entry{
	{
		{Name: "Academy Manufactor", Count: 1}, {Name: "Overencumbered", Count: 3}, {Name: "Thraben Inspector", Count: 4},
		{Name: "Strike It Rich", Count: 3}, {Name: "Dragon Fodder", Count: 2},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Glory Seeker", Count: 4},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Academy Manufactor", Count: 1}, {Name: "Overencumbered", Count: 2}, {Name: "Thraben Inspector", Count: 4},
		{Name: "Strike It Rich", Count: 3}, {Name: "Dragon Fodder", Count: 3},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Glory Seeker", Count: 3},
		{Name: "Plains", Count: 6}, {Name: "Mountain", Count: 5}, {Name: "Forest", Count: 5},
	},
}

// demoTokenscriptListGames plays the seeds and returns the clean games, the
// divergent ones and the counts of what fired.
func demoTokenscriptListGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, st *dirac.TokListStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoTokenscriptListPool, demoTokenscriptListDecks)
	st = &dirac.TokListStats{}
	dirac.DebugTokListStats = st
	defer func() { dirac.DebugTokListStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoTokenscriptListPool, demoTokenscriptListDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, d, st
}

// TestDemoTokenscriptListLockstep: no divergence and no kernel stall over 64
// seeds, and across them a list rewriter replaced a token and Overencumbered
// made its tokens for the enchanted player.
func TestDemoTokenscriptListLockstep(t *testing.T) {
	clean, divs, st := demoTokenscriptListGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Rewrites == 0 {
		t.Error("no game replaced a token with a list rewriter's cards")
	}
	if st.EnchantedMints == 0 {
		t.Error("no game made Overencumbered's tokens for the enchanted player")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoTokenscriptListMutation: with the list rewriters ignored every
// Clue, Food or Treasure keeps its own script, and some game diverges from
// gorge.
func TestDemoTokenscriptListMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugTokListRewriteOff = true
	defer func() { dirac.DebugTokListRewriteOff = false }()
	_, divs, _ := demoTokenscriptListGames(t, 64, lockstepQuiet)
	if len(divs) == 0 {
		t.Fatal("DebugTokListRewriteOff: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestFourSeatDemoTokenscriptListLockstep: four seats, gorge leading.
func TestFourSeatDemoTokenscriptListLockstep(t *testing.T) {
	a := pod4Deck(deckOf(1, "Academy Manufactor"), deckOf(3, "Overencumbered"), deckOf(4, "Thraben Inspector"),
		deckOf(3, "Strike It Rich"), deckOf(2, "Dragon Fodder"),
		deckOf(8, "Grizzly Bears"), deckOf(4, "Glory Seeker"), deckOf(6, "Plains"), deckOf(4, "Mountain"), deckOf(5, "Forest"))
	b := pod4Deck(deckOf(1, "Academy Manufactor"), deckOf(2, "Overencumbered"), deckOf(4, "Thraben Inspector"),
		deckOf(3, "Strike It Rich"), deckOf(3, "Dragon Fodder"),
		deckOf(8, "Grizzly Bears"), deckOf(3, "Glory Seeker"), deckOf(6, "Plains"), deckOf(5, "Mountain"), deckOf(5, "Forest"))
	if n := pod4FixtureLockstep(t, demoTokenscriptListPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
