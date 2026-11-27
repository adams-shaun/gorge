package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Thundermane lockstep: Thundermane Dragon's library-top permission may
// cast a power-4-or-greater creature spell, and the card's Static$ True
// SpellCast trigger resolves off the stack before the CR 603.3b ordering ask.
// The trigger's only body, DB$ Animate | Defined$ TriggeredCardLKICopy |
// Keywords$ Haste, is a no-op for the permanent: gorge applies the timed
// Animate to the spell and ends it when the spell leaves the stack
// (engine/effects/combatfx.go registerAnimateEffects, ExileOnMoved$ Stack), so
// the kernel drops the body to mirror gorge (dirac/kgen/
// lower_statictrigger_spellcast.go). The pool is
// dirac/cardset/pools/fixtures/demothundermane. StaticNoopStats counts the
// dropped-rider static triggers the games resolved, so a green run is not one
// where the trigger never fired.
const demoThundermanePool = "demothundermane"

var demoThundermaneDecks = [2][]cardset.Entry{
	{
		{Name: "Thundermane Dragon", Count: 4}, {Name: "Colossal Dreadmaw", Count: 4}, {Name: "Craw Wurm", Count: 4},
		{Name: "Earth Elemental", Count: 4}, {Name: "Fire Elemental", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 16},
	},
	{
		{Name: "Thundermane Dragon", Count: 4}, {Name: "Colossal Dreadmaw", Count: 4}, {Name: "Craw Wurm", Count: 4},
		{Name: "Earth Elemental", Count: 4}, {Name: "Fire Elemental", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8},
	},
}

// demoThundermaneGames plays the seeds and returns the clean games, the
// divergent ones and the count of dropped-rider static triggers resolved.
func demoThundermaneGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, resolved, stacked int64) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoThundermanePool, demoThundermaneDecks)
	before, beforeStacked := dirac.StaticNoopStats.Resolved.Load(), dirac.StaticNoopStats.Stacked.Load()
	c, _, d := lockstepSeedsMode(t, reg, set, demoThundermanePool, demoThundermaneDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, d, dirac.StaticNoopStats.Resolved.Load() - before, dirac.StaticNoopStats.Stacked.Load() - beforeStacked
}

// TestDemoThundermaneLockstep: no divergence and no kernel stall over 64
// seeds, and across them the dropped-rider static trigger resolved.
func TestDemoThundermaneLockstep(t *testing.T) {
	clean, divs, resolved, _ := demoThundermaneGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if resolved == 0 {
		t.Error("no game resolved Thundermane Dragon's static SpellCast trigger")
	}
	t.Logf("%d clean games; %d static no-op triggers", clean, resolved)
}

// TestDemoThundermaneMutation: with the empty-body static trigger forced onto
// the stack, the off-stack resolution the card lowers to is gone, and some
// game diverges from gorge.
func TestDemoThundermaneMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugStaticNoopOnStack = true
	defer func() { dirac.DebugStaticNoopOnStack = false }()
	_, divs, _, stacked := demoThundermaneGames(t, 64, lockstepQuiet)
	if stacked == 0 {
		t.Fatal("DebugStaticNoopOnStack: no dropped-rider static trigger reached the mutation")
	}
	if len(divs) == 0 {
		t.Fatal("DebugStaticNoopOnStack: no divergence in 64 games")
	}
	t.Logf("mutation forced %d triggers onto the stack; diverged in %d games", stacked, len(divs))
}

// TestFourSeatDemoThundermaneLockstep: four seats, gorge leading.
func TestFourSeatDemoThundermaneLockstep(t *testing.T) {
	a := pod4Deck(deckOf(4, "Thundermane Dragon"), deckOf(4, "Colossal Dreadmaw"), deckOf(4, "Craw Wurm"),
		deckOf(4, "Earth Elemental"), deckOf(4, "Fire Elemental"), deckOf(4, "Grizzly Bears"), deckOf(16, "Mountain"))
	b := pod4Deck(deckOf(4, "Thundermane Dragon"), deckOf(4, "Colossal Dreadmaw"), deckOf(4, "Craw Wurm"),
		deckOf(4, "Earth Elemental"), deckOf(4, "Fire Elemental"), deckOf(4, "Grizzly Bears"),
		deckOf(8, "Mountain"), deckOf(8, "Forest"))
	if n := pod4FixtureLockstep(t, demoThundermanePool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
