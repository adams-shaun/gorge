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

// Picker answers a decision: it returns one of d's options.
type Picker interface{ Pick(d *Decision) Action }

// RolloutResult is how one rollout ended. Exactly one of Over (a real game
// end, Winner -1 a draw), Stalled (the kernel hit a capacity or a rule gap:
// the position has no result, never a draw) and Truncated (the decision cap
// ran out) holds.
type RolloutResult struct {
	Over      bool
	Stalled   bool
	Truncated bool
	Winner    int8
	Reason    uint8 // StallReason when Stalled
	Decisions int   // decisions answered
}

// Rollout plays forks of a State to the end. It owns one working State, reused
// by every rollout, so a rollout allocates only the first time (and when a
// source needs a bigger cold buffer). One goroutine at a time.
type Rollout struct {
	work   State
	ready  bool
	dec    Decision    // Play's decision: on the heap once, since it goes through an interface
	Stalls [256]uint32 // stalled rollouts by StallReason
	Runs   uint32      // rollouts run
}

// Fork makes the working State a copy of src (State.CopyFrom) and returns it.
// src is never written. The fork is valid until the next Fork or Run.
func (r *Rollout) Fork(src *State) *State {
	if !r.ready {
		r.work.UseForkPool(NewForkPool())
		r.ready = true
	}
	r.work.CopyFrom(src)
	return &r.work
}

// Run forks src and plays the fork with pol until the game ends, the kernel
// stalls, or max decisions have been answered.
func (r *Rollout) Run(src *State, pol Picker, max int) RolloutResult {
	res := r.Play(r.Fork(src), pol, max)
	r.Runs++
	if res.Stalled {
		r.Stalls[res.Reason]++
	}
	return res
}

// Play steps s, answering with pol, until it ends, stalls or max decisions.
// It allocates nothing.
func (r *Rollout) Play(s *State, pol Picker, max int) RolloutResult {
	d := &r.dec
	var res RolloutResult
	for res.Decisions < max {
		s.StepInto(d)
		if d.Kind == DkNone {
			return finishRollout(s, res)
		}
		s.Apply(pol.Pick(d))
		res.Decisions++
		if s.over || s.g.stalled != 0 {
			return finishRollout(s, res)
		}
	}
	res.Truncated = true
	return res
}

func finishRollout(s *State, res RolloutResult) RolloutResult {
	if s.Stalled() {
		res.Stalled, res.Reason = true, s.StallReason()
		return res
	}
	res.Over, res.Winner = s.Terminal()
	return res
}
