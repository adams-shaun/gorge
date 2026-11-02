package dirac

import (
	"math/bits"
	"math/rand/v2"
)

// rng is gorge's engine RNG (engine/rules/rng.go) as a value: math/rand/v2's
// PCG seeded with (seed, seed^0x9e3779b97f4a7c15), drawn through the same
// IntN reduction rand.Rand uses (Lemire's multiply-shift with rejection),
// and a draw counter that counts IntN calls exactly as gorge's rng.Draws.
// rand.PCG is two uint64s, so the State stays pointer-free and a clone is a
// plain assignment.
type rng struct {
	pcg   rand.PCG
	Draws uint64
	// chance recording (Config.RecordChance): the bound of every draw, in
	// order, as runs of descending bounds (a shuffle of n cards draws
	// IntN(n), IntN(n-1), ..., IntN(2): one run); values are recomputed
	// from seed (ChanceTranscript).
	seed    uint64
	rec     bool
	recOver bool
	nr      uint8
	runs    [MaxChanceRuns]chanceRun
}

// chanceRun is draws with bounds first, first-1, ..., first-n+1.
type chanceRun struct{ first, n uint8 }

// MaxChanceRuns bounds a recorded chance transcript (draws past it set the
// overflow flag and are not recorded).
const MaxChanceRuns = 64

func newRNG(seed uint64) rng {
	var r rng
	r.pcg.Seed(seed, seed^0x9e3779b97f4a7c15)
	r.seed = seed
	return r
}

// IntN returns a value in [0,n), consuming one draw (gorge rng.IntN).
func (r *rng) IntN(n int) int {
	r.Draws++
	if r.rec {
		switch {
		case n >= 256:
			r.recOver = true
		case r.nr > 0 && r.runs[r.nr-1].n < 255 && int(r.runs[r.nr-1].first)-int(r.runs[r.nr-1].n) == n:
			r.runs[r.nr-1].n++
		case int(r.nr) < MaxChanceRuns:
			r.runs[r.nr] = chanceRun{first: uint8(n), n: 1}
			r.nr++
		default:
			r.recOver = true
		}
	}
	return r.intN(n)
}

func (r *rng) intN(n int) int {
	un := uint64(n)
	if un&(un-1) == 0 {
		return int(r.pcg.Uint64() & (un - 1))
	}
	hi, lo := bits.Mul64(r.pcg.Uint64(), un)
	if lo < un {
		thresh := -un % un
		for lo < thresh {
			hi, lo = bits.Mul64(r.pcg.Uint64(), un)
		}
	}
	return int(hi)
}

// ChanceDraw is one random outcome: IntN(Bound) returned Value (gorge's
// rules.ChanceDraw shape).
type ChanceDraw struct {
	Bound int
	Value int
}

// ChanceTranscript returns every random draw of the game so far (shuffle
// steps, the toss, random discards), in order, when the game was built
// with Config.RecordChance; ok is false when recording was off or the
// transcript overflowed MaxChanceRuns runs. It allocates (not for hot paths).
func (s *State) ChanceTranscript() (out []ChanceDraw, ok bool) {
	r := &s.rng
	if !r.rec || r.recOver {
		return nil, false
	}
	re := newRNG(r.seed)
	for i := 0; i < int(r.nr); i++ {
		ru := r.runs[i]
		for j := 0; j < int(ru.n); j++ {
			b := int(ru.first) - j
			out = append(out, ChanceDraw{Bound: b, Value: re.intN(b)})
		}
	}
	return out, true
}
