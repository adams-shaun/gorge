package dirac

// RandomPolicy is a tiny deterministic random policy (splitmix64) for
// drivers, benchmarks and soak tests.
type RandomPolicy struct{ x uint64 }

func NewRandomPolicy(seed uint64) RandomPolicy { return RandomPolicy{x: seed} }

func (r *RandomPolicy) next() uint64 {
	r.x += 0x9e3779b97f4a7c15
	z := r.x
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

// Pick chooses one of d's options uniformly.
func (r *RandomPolicy) Pick(d *Decision) Action {
	return d.Options[int(r.next()%uint64(d.N))]
}

// PlayRandom plays s to the end (or maxDecisions decisions) with policy
// pol. It returns the number of decisions answered and whether the game
// ended.
func PlayRandom(s *State, pol *RandomPolicy, maxDecisions int) (int, bool) {
	var d Decision
	n := 0
	for n < maxDecisions {
		s.StepInto(&d)
		if d.Kind == DkNone {
			return n, true
		}
		s.Apply(pol.Pick(&d))
		n++
	}
	return n, false
}
