package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Urza's Saga lockstep: Chapter III's "Search your library for an
// artifact card with mana cost {0} or {1}" (Artifact.ManaCost0,
// Artifact.ManaCost1), over the fixture pool dirac/cardset/pools/fixtures/urzasaga.
// gorge leads: the kernel must offer the same library cards. Ornithopter ({0})
// and Sol Ring ({1}) are the targets; Seat of the Synod prints no mana cost
// (mana value 0, but no cost {0}) and Mind Stone and Commander's Sphere cost
// more, so a read that ignored the printed cost or the value would offer a
// different list. ManaCostWordStats proves the word was read and matched.
const urzaSagaPool = "urzasaga"

var urzaSagaDecks = [2][]cardset.Entry{
	{
		{Name: "Urza's Saga", Count: 8}, {Name: "Ornithopter", Count: 5}, {Name: "Sol Ring", Count: 5},
		{Name: "Seat of the Synod", Count: 3}, {Name: "Mind Stone", Count: 3}, {Name: "Commander's Sphere", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Urza's Saga", Count: 8}, {Name: "Ornithopter", Count: 5}, {Name: "Sol Ring", Count: 5},
		{Name: "Seat of the Synod", Count: 3}, {Name: "Mind Stone", Count: 3}, {Name: "Commander's Sphere", Count: 3},
		{Name: "Forest", Count: 7}, {Name: "Island", Count: 6},
	},
}

// TestDemoUrzaSagaLockstep: no divergence over 64 seeds and, across them, the
// ManaCost<N> read ran and found a card.
func TestDemoUrzaSagaLockstep(t *testing.T) {
	t.Parallel()
	reads := dirac.ManaCostWordStats.Reads.Load()
	hits := dirac.ManaCostWordStats.Hits.Load()
	lockstepSet(t, urzaSagaPool, urzaSagaDecks, 64, "KURZASAGA_SEEDS")
	if dirac.ManaCostWordStats.Reads.Load() == reads {
		t.Fatal("no ManaCost<N> read: the pool does not reach Urza's Saga Chapter III")
	}
	if dirac.ManaCostWordStats.Hits.Load() == hits {
		t.Fatal("no ManaCost<N> read matched: the pool never found an artifact to fetch")
	}
	t.Logf("ManaCost<N>: %d reads, %d hits", dirac.ManaCostWordStats.Reads.Load(), dirac.ManaCostWordStats.Hits.Load())
}

// TestDemoUrzaSagaMutation: with the ManaCost<N> read disabled (it matches
// nothing), some game diverges from gorge.
func TestDemoUrzaSagaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoManaCostWord = true
	defer func() { dirac.DebugNoManaCostWord = false }()
	if lockstepDiverges(t, urzaSagaPool, urzaSagaDecks, 64) {
		return
	}
	t.Fatal("no seed diverged with the ManaCost<N> read disabled: the pool does not exercise Urza's Saga Chapter III")
}

// TestFourSeatDemoUrzaSagaLockstep: four seats, gorge leading.
func TestFourSeatDemoUrzaSagaLockstep(t *testing.T) {
	a := pod4Deck(deckOf(8, "Urza's Saga"), deckOf(5, "Ornithopter"), deckOf(5, "Sol Ring"), deckOf(3, "Seat of the Synod"),
		deckOf(3, "Mind Stone"), deckOf(3, "Commander's Sphere"), deckOf(7, "Forest"), deckOf(6, "Island"))
	b := pod4Deck(deckOf(8, "Urza's Saga"), deckOf(5, "Ornithopter"), deckOf(5, "Sol Ring"), deckOf(3, "Seat of the Synod"),
		deckOf(3, "Mind Stone"), deckOf(3, "Commander's Sphere"), deckOf(7, "Forest"), deckOf(6, "Island"))
	if n := pod4FixtureLockstep(t, urzaSagaPool, [4][]string{a, b, a, b}, 30, true, nil); n != 0 {
		t.Fatalf("%d diverged", n)
	}
}
