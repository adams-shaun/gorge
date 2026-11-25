package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A library-bound ChangeZoneAll that a CR 903.9b commander ask parks has moved
// the cards before the ask but not yet placed them: gorge's MoveZone appends
// each to the BOTTOM of its owner's library, and the sweep's tail, after the
// answer, puts the moved cards on top (libraryOrderPlacement) and shuffles.
// The kernel placed them on top at the park, so every card of the library sat
// one or more positions deeper than gorge's at the ask (Pos in the
// projection), with a differing pre-shuffle order. The kernel now keeps a
// parked sweep's cards at the bottom and rotates them to the top at the tail
// (czallCarry.bottom, finish).
//
// Two four-seat edh pods (kfuzz -kernel edh -seats 4): The Mending of
// Dominaria's chapter III shuffling a graveyard holding the commander's
// neighbour into the library (decision 1880) and Midnight Clock's hand and
// graveyard sweep (decision 4233). Their tables are the edh tables over their
// own four decks, which no glob helper builds, so they replay here.
var parkedSweepRepros = []struct {
	name string
	at   int
}{
	{"parked_sweep_mending_library_order", 1880}, // 1879 before the trample division ask at 924
	{"parked_sweep_midnight_clock_library_order", 4233},
}

func parkedSweepReplay(t *testing.T, reg *cards.Registry, name string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "one-off" || len(r.MoreCards) != 2 {
		t.Fatalf("pool %q, %d more decks: want a one-off four-seat pod", r.Pool, len(r.MoreCards))
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	if m.NSeat() != 4 {
		t.Fatalf("%d seats, want 4", m.NSeat())
	}
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(reg, set, names))
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

func parkedSweepLockstep(t *testing.T, i int) {
	t.Helper()
	if isolate(t) {
		return
	}
	c := parkedSweepRepros[i]
	res := parkedSweepReplay(t, synthReg(t), c.name)
	if res.Div != nil {
		t.Errorf("%s: divergence at %d: %s %s %v", c.name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= c.at {
		t.Errorf("%s: %d decisions, the game must go past decision %d", c.name, res.Decisions, c.at)
	}
}

// TestFourSeatParkedSweepMendingLockstep and ...MidnightClockLockstep: each pod
// replays clean past the decision where the kernel's library order differed at
// the commander ask (one test each: a pod is half the test budget).
func TestFourSeatParkedSweepMendingLockstep(t *testing.T)       { parkedSweepLockstep(t, 0) }
func TestFourSeatParkedSweepMidnightClockLockstep(t *testing.T) { parkedSweepLockstep(t, 1) }
