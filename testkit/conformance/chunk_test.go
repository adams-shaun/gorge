package conformance

import (
	"fmt"
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// maxChunkPairs keeps one chunk inside the test budget.
const maxChunkPairs = 20

// chunkWires are the wires a nightly chunk plays, by the name a found row uses.
var chunkWires = map[string]Wire{"native": Native{}, "manabrew": ManaBrew{}}

// chunkWireOrder is chunkWires in the order a chunk plays them.
var chunkWireOrder = []string{"native", "manabrew"}

// chunkSets are the deck pools a pair is dealt from, by the name a found row uses.
var chunkSets = map[string][][2]string{"fdn": fdnPairs}

// pairConfig is the game a (set, seed, pair) names: the deck pair at that place in
// the set, on a seed no other (seed, pair) has.
func pairConfig(t *testing.T, reg *cards.Registry, set string, seed uint64, pair int) game.Config {
	t.Helper()
	pairs, ok := chunkSets[set]
	if !ok {
		t.Fatalf("no deck set %q", set)
	}
	names := pairs[pair%len(pairs)]
	return game.Config{
		Seed:   seed<<20 | uint64(pair),
		Names:  []string{names[0], names[1]},
		Decks:  [][]*cards.Card{fdnDeck(t, reg, names[0]), fdnDeck(t, reg, names[1])},
		Tokens: reg.Tokens,
	}
}

func chunkEnv(t *testing.T, name string) (uint64, bool) {
	t.Helper()
	s, ok := os.LookupEnv(name)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		t.Fatalf("%s=%q: %v", name, s, err)
	}
	return n, true
}

// TestConformanceChunk plays one chunk of a nightly run: CONFORMANCE_PAIRS games
// from pair CONFORMANCE_FIRST on seed CONFORMANCE_SEED, on each wire. Each diff
// prints a line starting FOUND, which scripts/wire-nightly.sh collects. It skips
// when unset, and the script fails on a skip, so a nightly run is many tests that
// each fit the test budget.
func TestConformanceChunk(t *testing.T) {
	seed, okSeed := chunkEnv(t, "CONFORMANCE_SEED")
	first, okFirst := chunkEnv(t, "CONFORMANCE_FIRST")
	count, okPairs := chunkEnv(t, "CONFORMANCE_PAIRS")
	if !okSeed || !okFirst || !okPairs {
		t.Skip("CONFORMANCE_SEED, CONFORMANCE_FIRST and CONFORMANCE_PAIRS are unset")
	}
	if count > maxChunkPairs {
		t.Fatalf("CONFORMANCE_PAIRS=%d is over the chunk size %d", count, maxChunkPairs)
	}
	reg := testutil.CorpusRegistry(t)
	for _, wire := range chunkWireOrder {
		for pair := int(first); pair < int(first+count); pair++ {
			t.Run(fmt.Sprintf("%s/pair%d", wire, pair), func(t *testing.T) {
				t.Parallel()
				row := foundRow{wire: wire, set: "fdn", seed: seed, pair: pair}
				t.Cleanup(func() {
					if t.Failed() {
						fmt.Printf("FOUND\t%s\n", row)
					}
				})
				if err := row.check(t, reg, gorgegame.Engine{}, gorgegame.Engine{}); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
