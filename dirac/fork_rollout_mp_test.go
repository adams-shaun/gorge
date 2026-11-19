package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/vanilla"
)

func rolloutRoot(seed uint64, n uint8) *State {
	var decks [MaxSeats][]CardID
	for p := 0; p < int(n); p++ {
		decks[p] = vanilla.Decks[(int(seed)+p)%5]
	}
	s := new(State)
	s.InitN(vanilla.Table, decks, n, seed, Config{Mulligans: 1})
	return s
}

// advance plays k random decisions so the root is mid-game.
func advance(s *State, seed uint64, k int) {
	pol := NewRandomPolicy(seed)
	PlayRandom(s, &pol, k)
}

func TestForkLeavesOriginal(t *testing.T) {
	root := rolloutRoot(3, 4)
	advance(root, 9, 60)
	want := new(State)
	want.CopyFrom(root)
	var r Rollout
	for k := uint64(0); k < 8; k++ {
		pol := NewRandomPolicy(100 + k)
		res := r.Run(root, &pol, 100000)
		if !res.Over {
			t.Fatalf("rollout %d did not end: %+v", k, res)
		}
		if !root.SameGame(want) {
			t.Fatalf("root changed after rollout %d", k)
		}
	}
}

func TestRolloutFourSeatsEnds(t *testing.T) {
	var r Rollout
	for seed := uint64(0); seed < 6; seed++ {
		root := rolloutRoot(seed, 4)
		pol := NewRandomPolicy(seed)
		res := r.Run(root, &pol, 100000)
		if !res.Over || res.Stalled || res.Truncated || res.Decisions == 0 {
			t.Fatalf("seed %d: %+v", seed, res)
		}
		if res.Winner < -1 || res.Winner >= 4 {
			t.Fatalf("seed %d: winner %d", seed, res.Winner)
		}
	}
}

func TestRolloutStallIsCountedNotDraw(t *testing.T) {
	root := rolloutRoot(5, 4)
	var r Rollout
	s := r.Fork(root)
	pol := NewRandomPolicy(1)
	if res := r.Play(s, &pol, 40); !res.Truncated {
		t.Fatalf("setup: %+v", res)
	}
	s.stall(StallBoard) // the kernel latches a capacity stall mid-game
	res := r.Play(s, &pol, 100000)
	if !res.Stalled || res.Over || res.Truncated || res.Reason != StallBoard {
		t.Fatalf("want a StallBoard stall, got %+v", res)
	}
	// Run counts a stall by reason: a source already stalled stalls its forks.
	var r2 Rollout
	if res := r2.Run(s, &pol, 100); !res.Stalled {
		t.Fatalf("fork of a stalled state: %+v", res)
	}
	if r2.Stalls[StallBoard] != 1 || r2.Runs != 1 {
		t.Fatalf("stalls %d runs %d", r2.Stalls[StallBoard], r2.Runs)
	}
}

func TestRolloutTruncates(t *testing.T) {
	var r Rollout
	pol := NewRandomPolicy(1)
	res := r.Run(rolloutRoot(1, 4), &pol, 5)
	if !res.Truncated || res.Over || res.Decisions != 5 {
		t.Fatalf("%+v", res)
	}
}

func TestRolloutDeterministic(t *testing.T) {
	root := rolloutRoot(7, 4)
	advance(root, 2, 30)
	var a, b Rollout
	p1, p2 := NewRandomPolicy(55), NewRandomPolicy(55)
	r1, r2 := a.Run(root, &p1, 100000), b.Run(root, &p2, 100000)
	if r1 != r2 || !a.work.SameGame(&b.work) {
		t.Fatalf("%+v vs %+v", r1, r2)
	}
}

func TestRolloutStepAllocsZero(t *testing.T) {
	root := rolloutRoot(4, 4)
	advance(root, 3, 40)
	var r Rollout
	s := r.Fork(root) // warm the buffers
	pol := NewRandomPolicy(8)
	var d Decision
	got := testing.AllocsPerRun(50, func() {
		s = r.Fork(root)
		for i := 0; i < 20; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				break
			}
			s.Apply(pol.Pick(&d))
		}
	})
	if got != 0 {
		t.Fatalf("fork + 20 steps allocated %v", got)
	}
	got = testing.AllocsPerRun(20, func() { r.Play(r.Fork(root), &pol, 1000) })
	if got != 0 {
		t.Fatalf("whole rollout allocated %v", got)
	}
}
