package host

import (
	"fmt"
	"math/rand/v2"
)

// MatchSeed derives match k's engine seed from its table's seed with
// splitmix64's finaliser over tableSeed XOR k·φ, so a table's whole history
// is a pure function of its configuration (spec D14) and consecutive
// matches share nothing but the table.
func MatchSeed(tableSeed uint64, k int) uint64 {
	z := tableSeed ^ (uint64(k) * 0x9E3779B97F4A7C15)
	z = (z ^ (z >> 30)) * 0xBF58476D1CE4E5B9
	z = (z ^ (z >> 27)) * 0x94D049BB133111EB
	return z ^ (z >> 31)
}

// SeededShuffle returns in reordered by a deterministic Fisher-Yates keyed
// on seed, so a caller can assign decks "at random" and have the assignment
// be a pure function of the seed: the same seed reproduces the same order,
// which is what keeps an on-demand game replayable from its configuration.
// It is the one place in host that turns a seed into an ordering; the game
// creator (cmd/gorged) shuffles a format's deck pool with the game's own
// seed, and the resulting order is persisted verbatim in TableConfig.Decks,
// so both the dealt decks and the match are reproducible from the recorded
// config. in is not modified.
func SeededShuffle(seed uint64, in []string) []string {
	out := append([]string(nil), in...)
	rng := rand.New(rand.NewPCG(seed, seed^0x9E3779B97F4A7C15))
	for i := len(out) - 1; i > 0; i-- {
		j := rng.IntN(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// NextGameID returns the smallest free "g<N>" table id, scanning the
// registry's tables so an on-demand game created before a restart never
// collides with one loaded back from tables.json (R-E0: a table id is the
// registry key, and AddTable rejects a duplicate). Exported for the game
// creator (cmd/gorged), which allocates ids when a browser asks for a
// fresh game.
func NextGameID(reg *Registry) TableID {
	max := 0
	for _, t := range reg.Tables() {
		var n int
		if _, err := fmt.Sscanf(t.ID, "g%d", &n); err == nil && n > 0 && n > max {
			max = n
		}
	}
	return TableID(fmt.Sprintf("g%d", max+1))
}
