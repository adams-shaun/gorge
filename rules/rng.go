package rules

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/state"
)

// rng is the engine's only source of randomness. Draws is counted so a replay
// can assert it consumed exactly the same number of values — the cheapest
// possible detector for "the engine changed underneath this log".
type rng struct {
	src   *rand.Rand
	Draws uint64
	seed  [2]uint64
}

func newRNG(seed uint64) *rng {
	s := [2]uint64{seed, seed ^ 0x9e3779b97f4a7c15}
	return &rng{src: rand.New(rand.NewPCG(s[0], s[1])), seed: s}
}

func (r *rng) IntN(n int) int {
	r.Draws++
	return r.src.IntN(n)
}

// Shuffle is Fisher-Yates and consumes exactly len(ids)-1 values, so draw count
// is a pure function of deck size.
func (r *rng) Shuffle(ids []state.ObjID) {
	for i := len(ids) - 1; i > 0; i-- {
		j := r.IntN(i + 1)
		ids[i], ids[j] = ids[j], ids[i]
	}
}
