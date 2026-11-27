package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The TokenAttacking$ demo lockstep: Flamerush Rider copies another attacker
// tapped and attacking (CopyPermanent TokenAttacking$ True beside
// ValidTgts$), Shaun, Father of Synths copies an attacking legendary creature
// from its AttackersDeclared trigger, and Sigiled Sword of Valeron's
// equipped-creature Attacks trigger makes an attacking Knight token (Token
// TokenAttacking$ True outside the source's own attack). The pool is
// dirac/cardset/pools/fixtures/tokenattacking. The kernel's
// TokenAttackingStats count the copies and the equipped-trigger tokens that
// entered attacking, so a green game is not one where no shape was reached.
const tokenattackingPool = "tokenattacking"

var tokenattackingDecks = [2][]cardset.Entry{
	{
		{Name: "Flamerush Rider", Count: 4}, {Name: "Shaun, Father of Synths", Count: 4},
		{Name: "Sigiled Sword of Valeron", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Isamaru, Hound of Konda", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Island", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Glory Seeker", Count: 8},
		{Name: "Shock", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 4},
	},
}

// tokenattackingGames plays the seeds and returns the clean games, the
// divergent ones and the shape counts.
func tokenattackingGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, st *dirac.TokenAttackingStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, tokenattackingPool, tokenattackingDecks)
	st = &dirac.TokenAttackingStats{}
	dirac.DebugTokenAttackingStats = st
	defer func() { dirac.DebugTokenAttackingStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, tokenattackingPool, tokenattackingDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, d, st
}

// TestDemoTokenattackingLockstep: no divergence and no kernel stall over 64
// seeds, and across them both new shapes fired.
func TestDemoTokenattackingLockstep(t *testing.T) {
	clean, divs, st := tokenattackingGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Copies == 0 {
		t.Error("no token copy entered attacking (CopyPermanent TokenAttacking$ True)")
	}
	if st.EquippedTokens == 0 {
		t.Error("no equipped-creature attack made an attacking token (Sigiled Sword of Valeron)")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoTokenattackingMutation: with the new copy read disabled, some game
// diverges from gorge.
func TestDemoTokenattackingMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoCopyAttacking = true
	defer func() { dirac.DebugNoCopyAttacking = false }()
	_, divs, _ := tokenattackingGames(t, 64, lockstepQuiet)
	if len(divs) == 0 {
		t.Fatal("DebugNoCopyAttacking: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestFourSeatDemoTokenattackingLockstep: four seats, gorge leading. The
// copy's CR 508.4 defender ask (players only) and the trigger's captured
// defending player are exercised past two seats.
func TestFourSeatDemoTokenattackingLockstep(t *testing.T) {
	a := pod4Deck(deckOf(4, "Flamerush Rider"), deckOf(4, "Shaun, Father of Synths"), deckOf(4, "Sigiled Sword of Valeron"),
		deckOf(4, "Calamity, Galloping Inferno"), deckOf(4, "Savannah Lions"), deckOf(4, "Isamaru, Hound of Konda"),
		deckOf(4, "Shock"), deckOf(8, "Plains"), deckOf(4, "Island"), deckOf(4, "Mountain"))
	b := pod4Deck(deckOf(8, "Grizzly Bears"), deckOf(8, "Glory Seeker"), deckOf(4, "Shock"),
		deckOf(10, "Plains"), deckOf(6, "Forest"), deckOf(4, "Mountain"))
	st := &dirac.TokenAttackingStats{}
	dirac.DebugTokenAttackingStats = st
	defer func() { dirac.DebugTokenAttackingStats = nil }()
	if n := pod4FixtureLockstep(t, tokenattackingPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
	if st.Copies == 0 {
		t.Error("no token copy entered attacking in four seats")
	}
	if st.EquippedTokens == 0 {
		t.Error("no equipped-creature attack made an attacking token in four seats")
	}
	if st.Asks == 0 {
		t.Error("no copy posed its CR 508.4 defender ask in four seats")
	}
	if st.PickedAsks == 0 {
		t.Error("no ExtCopyPicked copy (Calamity, Galloping Inferno) posed a defender ask in four seats")
	}
	t.Logf("four-seat shapes: %+v", *st)
}
