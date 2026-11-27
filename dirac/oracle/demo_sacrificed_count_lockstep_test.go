package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The filtered sacrificed-this-turn lockstep: Dargo, the Shipwrecker's
// ReduceCost SVar reads PlayerCountPropertyYou$SacrificedThisTurn
// Artifact,Creature and Evendo Brushrazer's Continuous CheckSVar reads
// Permanent.!token. Both now lower to an ExSacrificedThisTurn filter
// (dirac/value_sacrificedthisturn.go, the turn's zone-entry ledger), so the
// pool makes the filter observable: Goblin Bombardment, Carrion Feeder and
// Hungry Ghoul sacrifice, Dragon Fodder and Krenko's Command supply token
// creatures (which Evendo's !token spec must skip and Dargo's union must
// count), Goldhound and Ornithopter are non-token artifacts, and Evendo's own
// {T}, Sacrifice a land ability is a non-token permanent sacrifice. The pool
// is dirac/cardset/pools/fixtures/sacrificedcount.
const demoSacrificedCountPool = "sacrificedcount"

var demoSacrificedCountDecks = [2][]cardset.Entry{
	{ // Dargo's deck
		{Name: "Dargo, the Shipwrecker", Count: 2}, {Name: "Goblin Bombardment", Count: 3},
		{Name: "Carrion Feeder", Count: 3}, {Name: "Hungry Ghoul", Count: 2},
		{Name: "Dragon Fodder", Count: 4}, {Name: "Krenko's Command", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Goldhound", Count: 3},
		{Name: "Ornithopter", Count: 2}, {Name: "Sol Ring", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 4}, {Name: "Forest", Count: 2},
	},
	{ // Evendo's deck
		{Name: "Evendo Brushrazer", Count: 3}, {Name: "Goblin Bombardment", Count: 3},
		{Name: "Dragon Fodder", Count: 4}, {Name: "Krenko's Command", Count: 4},
		{Name: "Goldhound", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Mountain", Count: 10},
	},
}

// demoSacrificedCountGames plays the seeds and returns the clean games, the
// divergent ones and the filtered-read counts.
func demoSacrificedCountGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, st *dirac.SacrificedCountStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoSacrificedCountPool, demoSacrificedCountDecks)
	st = &dirac.SacrificedCountStats{}
	dirac.DebugSacrificedCountStats = st
	defer func() { dirac.DebugSacrificedCountStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, demoSacrificedCountPool, demoSacrificedCountDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, d, st
}

// TestDemoSacrificedCountLockstep: no divergence and no kernel stall over 64
// two-seat seeds, and across them a filtered sacrifice read returned a
// nonzero count.
func TestDemoSacrificedCountLockstep(t *testing.T) {
	clean, divs, st := demoSacrificedCountGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if st.Reads == 0 {
		t.Error("no game evaluated a filtered SacrificedThisTurn read")
	}
	if st.NonZero == 0 {
		t.Error("no game counted a sacrifice through the filter")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoSacrificedCountMutation: with the new filter read disabled (every
// sacrifice counts, tokens and all) some game diverges from gorge.
func TestDemoSacrificedCountMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSacrificedReadOff = true
	defer func() { dirac.DebugSacrificedReadOff = false }()
	_, divs, _ := demoSacrificedCountGames(t, 64, lockstepQuiet)
	if len(divs) == 0 {
		t.Fatal("DebugSacrificedReadOff: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", len(divs))
}

// TestFourSeatDemoSacrificedCountLockstep: four seats, gorge leading.
func TestFourSeatDemoSacrificedCountLockstep(t *testing.T) {
	a := pod4Deck(deckOf(2, "Dargo, the Shipwrecker"), deckOf(3, "Goblin Bombardment"), deckOf(3, "Carrion Feeder"),
		deckOf(2, "Hungry Ghoul"), deckOf(4, "Dragon Fodder"), deckOf(3, "Krenko's Command"),
		deckOf(4, "Grizzly Bears"), deckOf(3, "Goldhound"), deckOf(2, "Ornithopter"), deckOf(2, "Sol Ring"),
		deckOf(6, "Mountain"), deckOf(4, "Swamp"), deckOf(2, "Forest"))
	b := pod4Deck(deckOf(3, "Evendo Brushrazer"), deckOf(3, "Goblin Bombardment"), deckOf(4, "Dragon Fodder"),
		deckOf(4, "Krenko's Command"), deckOf(4, "Goldhound"), deckOf(4, "Grizzly Bears"),
		deckOf(3, "Hill Giant"), deckOf(3, "Shock"), deckOf(2, "Lightning Bolt"), deckOf(10, "Mountain"))
	if n := pod4FixtureLockstep(t, demoSacrificedCountPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
