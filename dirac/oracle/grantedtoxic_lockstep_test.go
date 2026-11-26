package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The granted-toxic lockstep (CR 702.164c): Toxic N granted rather than
// printed. Aspirant's Ascent (Pump KW$) and Plague Nurse (PumpAll KW$) grant
// until end of turn, Karumonix's static grants the other Rats, Necrogen
// Communion and Prosthetic Injector grant to their host, and the kernel adds
// every grant to the printed N (dirac/kw_toxicgrant.go) as gorge's ToxicValue
// sums the derived keywords. The pool is dirac/cardset/pools/fixtures/
// grantedtoxic. The kernel's GrantedPoison count proves a grant decided a
// player's poison, so a green game is not one where no grant mattered.
const grantedToxicPool = "grantedtoxic"

var grantedToxicDecks = [2][]cardset.Entry{
	{
		{Name: "Karumonix, the Rat King", Count: 4}, {Name: "Gutter Skulk", Count: 4}, {Name: "Muck Rats", Count: 4}, {Name: "Necrogen Communion", Count: 4},
		{Name: "Prosthetic Injector", Count: 4}, {Name: "Plague Nurse", Count: 4}, {Name: "Branchblight Stalker", Count: 2}, {Name: "Monument to Perfection", Count: 2},
		{Name: "Swamp", Count: 6}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Aspirant's Ascent", Count: 6}, {Name: "Grizzly Bears", Count: 6}, {Name: "Glory Seeker", Count: 6}, {Name: "Branchblight Stalker", Count: 4},
		{Name: "Prosthetic Injector", Count: 2}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8},
	},
}

// grantedToxicGames plays the seeds and returns the clean games, the
// divergent ones and the poison the grants placed.
func grantedToxicGames(t *testing.T, hi uint64, mode lockstepMode) (clean, divs int, st *dirac.ToxicStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, grantedToxicPool, grantedToxicDecks)
	st = &dirac.ToxicStats{}
	dirac.DebugToxicStats = st
	defer func() { dirac.DebugToxicStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, grantedToxicPool, grantedToxicDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, len(d), st
}

// TestGrantedToxicLockstep: no divergence and no kernel stall over 64 seeds,
// and across them a granted toxic placed poison.
func TestGrantedToxicLockstep(t *testing.T) {
	clean, divs, st := grantedToxicGames(t, 64, lockstepFailEach)
	if divs > 0 {
		t.Errorf("%d divergent games", divs)
	}
	if st.GrantedPoison == 0 {
		t.Error("no game placed poison through a granted toxic")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestGrantedToxicMutation: with the reader ignoring every grant, some game
// diverges from gorge.
func TestGrantedToxicMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoGrantedToxic = true
	defer func() { dirac.DebugNoGrantedToxic = false }()
	_, divs, _ := grantedToxicGames(t, 64, lockstepQuiet)
	if divs == 0 {
		t.Fatal("DebugNoGrantedToxic: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", divs)
}

// TestFourSeatGrantedToxicLockstep: four seats, gorge leading.
func TestFourSeatGrantedToxicLockstep(t *testing.T) {
	a := pod4Deck(deckOf(4, "Karumonix, the Rat King"), deckOf(4, "Gutter Skulk"), deckOf(4, "Muck Rats"), deckOf(4, "Necrogen Communion"),
		deckOf(4, "Prosthetic Injector"), deckOf(4, "Plague Nurse"), deckOf(2, "Branchblight Stalker"), deckOf(2, "Monument to Perfection"),
		deckOf(6, "Swamp"), deckOf(6, "Forest"))
	b := pod4Deck(deckOf(6, "Aspirant's Ascent"), deckOf(6, "Grizzly Bears"), deckOf(6, "Glory Seeker"), deckOf(4, "Branchblight Stalker"),
		deckOf(2, "Prosthetic Injector"), deckOf(8, "Island"), deckOf(8, "Forest"))
	if n := pod4FixtureLockstep(t, grantedToxicPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
