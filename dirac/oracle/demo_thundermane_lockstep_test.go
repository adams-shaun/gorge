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
// Keywords$ Haste, is a timed grant on the triggering spell that gorge carries
// onto the permanent it becomes (CR 400.7a; engine/effects/combatfx.go
// ExileOnMovedStackCarry), so the kernel stamps the spell entry with the
// override record and applies it as the permanent enters
// (dirac/static_spellcarry.go). The pool is
// dirac/cardset/pools/fixtures/demothundermane. SpellCarryStats counts the
// records stamped and applied, so a green run is not one where the trigger
// never fired.
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
// divergent ones and the count of rider records stamped on a spell and applied
// as the permanent entered.
func demoThundermaneGames(t *testing.T, hi uint64, mode lockstepMode) (clean int, divs []lockstepDivergence, stamped, entered int64) {
	t.Helper()
	reg, set, gd := lockstepPool(t, demoThundermanePool, demoThundermaneDecks)
	beforeStamped, beforeEntered := dirac.SpellCarryStats.Stamped.Load(), dirac.SpellCarryStats.Entered.Load()
	c, _, d := lockstepSeedsMode(t, reg, set, demoThundermanePool, demoThundermaneDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, d, dirac.SpellCarryStats.Stamped.Load() - beforeStamped, dirac.SpellCarryStats.Entered.Load() - beforeEntered
}

// TestDemoThundermaneLockstep: no divergence and no kernel stall over 64
// seeds, and across them the rider was applied as the permanent entered.
func TestDemoThundermaneLockstep(t *testing.T) {
	clean, divs, stamped, entered := demoThundermaneGames(t, 64, lockstepFailEach)
	if len(divs) > 0 {
		t.Errorf("%d divergent games", len(divs))
	}
	if stamped == 0 || entered == 0 {
		t.Errorf("the rider was not applied: %d stamped, %d entered", stamped, entered)
	}
	t.Logf("%d clean games; %d rider records stamped, %d applied at entry", clean, stamped, entered)
}

// TestDemoThundermaneMutation: with the stamped rider not applied as the
// permanent enters, the haste gorge grants is gone, and some game diverges.
func TestDemoThundermaneMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSpellCarryOff = true
	defer func() { dirac.DebugSpellCarryOff = false }()
	_, divs, stamped, _ := demoThundermaneGames(t, 64, lockstepQuiet)
	if stamped == 0 {
		t.Fatal("DebugSpellCarryOff: no rider record was stamped")
	}
	if len(divs) == 0 {
		t.Fatal("DebugSpellCarryOff: no divergence in 64 games")
	}
	t.Logf("mutation dropped %d stamped rider records; diverged in %d games", stamped, len(divs))
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
