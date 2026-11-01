package draftlab

import "math/rand/v2"

// gameSaltBase tags the per-game seed streams. It is cmd/draftsim's
// gameSaltBase (cmd/draftsim/event.go), copied so a structured Play derives
// each game's engine seed exactly as draftsim's event games do: the lab and
// the reference driver then agree game-for-game.
const gameSaltBase = 0x6A111E

// gamesSaltBase tags the parallel Games batch's per-matchup streams
// (lab Games i = rand.NewPCG(seed, 0x1AB6A3<<32|i)).
const gamesSaltBase = 0x1AB6A3

// gameSeed derives one game's engine seed from a run seed and the game's
// place in the run: a pure function of (seed, round, a, b, game). It is
// cmd/draftsim/event.go's gameSeed, copied verbatim so Play pairs games with
// the same seeds draftsim's event driver would.
func gameSeed(seed uint64, round, a, b, game int) uint64 {
	r := rand.New(rand.NewPCG(seed, gameSaltBase<<32|
		uint64(round)<<24|uint64(a)<<16|uint64(b)<<8|uint64(game)))
	return r.Uint64()
}
