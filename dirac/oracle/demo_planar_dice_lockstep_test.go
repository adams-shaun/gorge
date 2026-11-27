package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The planar-die lockstep (CR 901.3): Fractured Powerstone's {T}: Roll the
// planar die and Ichor Elixir's "roll that many plus one and ignore one"
// replacement, the corpus's only two carriers, over the fixture pool
// dirac/cardset/pools/fixtures/planardice. gorge leads: the kernel must draw
// the same rng values in the same order (the extra die Ichor adds is drawn
// and dropped) and keep the same first count-ignore results. PlanarDiceStats
// proves the replacement decided a roll, so a green game is not one where the
// shape never fired.
const planarDicePool = "planardice"

var planarDiceDecks = [2][]cardset.Entry{
	{
		{Name: "Fractured Powerstone", Count: 8}, {Name: "Ichor Elixir", Count: 8},
		{Name: "Forest", Count: 12}, {Name: "Swamp", Count: 12},
	},
	{
		{Name: "Fractured Powerstone", Count: 8}, {Name: "Ichor Elixir", Count: 8},
		{Name: "Forest", Count: 12}, {Name: "Swamp", Count: 12},
	},
}

// TestDemoPlanarDiceLockstep: no divergence over 64 seeds and, across them,
// both a bare roll and a roll Ichor Elixir rewrote.
func TestDemoPlanarDiceLockstep(t *testing.T) {
	t.Parallel()
	rolls := dirac.PlanarDiceStats.Rolls.Load()
	replaced := dirac.PlanarDiceStats.Replaced.Load()
	extra := dirac.PlanarDiceStats.ExtraDice.Load()
	ignored := dirac.PlanarDiceStats.Ignored.Load()
	lockstepSet(t, planarDicePool, planarDiceDecks, 64, "KPLANAR_SEEDS")
	if dirac.PlanarDiceStats.Rolls.Load() == rolls {
		t.Fatal("no planar die was rolled: the pool does not exercise RollPlanarDice")
	}
	if dirac.PlanarDiceStats.Replaced.Load() == replaced {
		t.Fatal("no planar roll met a replacement: the pool does not exercise Ichor Elixir")
	}
	if dirac.PlanarDiceStats.ExtraDice.Load() == extra || dirac.PlanarDiceStats.Ignored.Load() == ignored {
		t.Fatal("the replacement added or ignored no die: the pool does not exercise its rewrite")
	}
	t.Logf("planar dice: %d rolls, %d replaced, %d extra dice, %d ignored",
		dirac.PlanarDiceStats.Rolls.Load(), dirac.PlanarDiceStats.Replaced.Load(),
		dirac.PlanarDiceStats.ExtraDice.Load(), dirac.PlanarDiceStats.Ignored.Load())
}

// TestDemoPlanarDiceMutation: with the replacement read disabled, some game
// diverges from gorge (the extra die the replacement draws moves the rng).
func TestDemoPlanarDiceMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoPlanarReplacement = true
	defer func() { dirac.DebugNoPlanarReplacement = false }()
	if lockstepDiverges(t, planarDicePool, planarDiceDecks, 64) {
		return
	}
	t.Fatal("no seed diverged with the planar-dice replacement disabled: the pool does not exercise Ichor Elixir")
}

// TestFourSeatDemoPlanarDiceLockstep: four seats, gorge leading.
func TestFourSeatDemoPlanarDiceLockstep(t *testing.T) {
	a := pod4Deck(deckOf(8, "Fractured Powerstone"), deckOf(8, "Ichor Elixir"), deckOf(12, "Forest"), deckOf(12, "Swamp"))
	b := pod4Deck(deckOf(8, "Fractured Powerstone"), deckOf(8, "Ichor Elixir"), deckOf(12, "Forest"), deckOf(12, "Swamp"))
	if n := pod4FixtureLockstep(t, planarDicePool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
