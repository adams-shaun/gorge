package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The metalcraft lockstep (CR 207.2c): Brotherhood Scribe's Metalcraft-gated
// non-mana activation and its energy-gain trigger (a PlayerCounterChange put),
// Urza's Workshop's plain {T}: Add {C} beside a Metalcraft-gated evaluated
// {T}: Add {C}, and Vedalken Humiliator's Metalcraft$ Attacks trigger. The
// pool is dirac/cardset/pools/fixtures/metalcraft; artifact lands and
// zero-cost artifact creatures bring the gate up, and four Workshops make the
// evaluated amount two or more. The kernel's MetalcraftStats count proves a
// green game is one where a metalcraft condition held, not one where the pool
// never reached three artifacts.
const metalcraftPool = "metalcraft"

var metalcraftDecks = [2][]cardset.Entry{
	{
		{Name: "Brotherhood Scribe", Count: 4}, {Name: "Urza's Workshop", Count: 4}, {Name: "Vedalken Humiliator", Count: 4},
		{Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Seat of the Synod", Count: 4}, {Name: "Ancient Den", Count: 4},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 6},
	},
	{
		{Name: "Brotherhood Scribe", Count: 2}, {Name: "Urza's Workshop", Count: 4}, {Name: "Vedalken Humiliator", Count: 2},
		{Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 4},
		{Name: "Seat of the Synod", Count: 4}, {Name: "Ancient Den", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Plains", Count: 8},
	},
}

// metalcraftGames plays the seeds and returns the clean games, the divergent
// ones and the metalcraft reads that held.
func metalcraftGames(t *testing.T, hi uint64, mode lockstepMode) (clean, divs int, st *dirac.MetalcraftStats) {
	t.Helper()
	reg, set, gd := lockstepPool(t, metalcraftPool, metalcraftDecks)
	st = &dirac.MetalcraftStats{}
	dirac.DebugMetalcraftStats = st
	defer func() { dirac.DebugMetalcraftStats = nil }()
	c, _, d := lockstepSeedsMode(t, reg, set, metalcraftPool, metalcraftDecks, gd, 1, hi, mode, nil, nil, castFirstPolicy)
	return c, len(d), st
}

// TestDemoMetalcraftLockstep: no divergence and no kernel stall over 64 seeds,
// and across them a metalcraft condition held (the activation, the trigger or
// the gated mana view).
func TestDemoMetalcraftLockstep(t *testing.T) {
	clean, divs, st := metalcraftGames(t, 64, lockstepFailEach)
	if divs > 0 {
		t.Errorf("%d divergent games", divs)
	}
	if st.CondHeld == 0 {
		t.Error("no game read a metalcraft condition true")
	}
	if st.EnergyTr == 0 {
		t.Error("no game queued Brotherhood Scribe's energy-gain trigger (TrigPlayerCounterAdded)")
	}
	t.Logf("%d clean games; %+v", clean, *st)
}

// TestDemoPlayerCtrMutation: with the energy-gain trigger silenced in the
// kernel, some game diverges from gorge (which queues it).
func TestDemoPlayerCtrMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugPlayerCtrOff = true
	defer func() { dirac.DebugPlayerCtrOff = false }()
	_, divs, _ := metalcraftGames(t, 64, lockstepQuiet)
	if divs == 0 {
		t.Fatal("DebugPlayerCtrOff: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", divs)
}

// TestDemoMetalcraftMutation: with every metalcraft read forced false, some
// game diverges from gorge.
func TestDemoMetalcraftMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugMetalcraftOff = true
	defer func() { dirac.DebugMetalcraftOff = false }()
	_, divs, _ := metalcraftGames(t, 64, lockstepQuiet)
	if divs == 0 {
		t.Fatal("DebugMetalcraftOff: no divergence in 64 games")
	}
	t.Logf("mutation diverged in %d games", divs)
}

// TestFourSeatDemoMetalcraftLockstep: four seats, gorge leading.
func TestFourSeatDemoMetalcraftLockstep(t *testing.T) {
	a := pod4Deck(
		deckOf(4, "Brotherhood Scribe"), deckOf(4, "Urza's Workshop"), deckOf(4, "Vedalken Humiliator"),
		deckOf(4, "Memnite"), deckOf(4, "Ornithopter"),
		deckOf(4, "Seat of the Synod"), deckOf(4, "Ancient Den"), deckOf(6, "Island"), deckOf(6, "Plains"))
	b := pod4Deck(
		deckOf(2, "Brotherhood Scribe"), deckOf(4, "Urza's Workshop"), deckOf(2, "Vedalken Humiliator"),
		deckOf(4, "Memnite"), deckOf(4, "Ornithopter"),
		deckOf(4, "Seat of the Synod"), deckOf(4, "Ancient Den"), deckOf(8, "Island"), deckOf(8, "Plains"))
	if n := pod4FixtureLockstep(t, metalcraftPool, [4][]string{a, b, a, b}, 32, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
