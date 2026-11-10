package search

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestRootEvalUnsetMatchesBase pins the search without a RootEval: the
// digests were recorded on the base commit before Searcher.RootEval
// existed, so any change that leaks into the unset path fails here.
func TestRootEvalUnsetMatchesBase(t *testing.T) {
	for _, tc := range []struct {
		name string
		o    Options
		want uint64
	}{
		{"plain", rootnetBaseOpts(false), 0x2590747fad8bed2d},
		{"cached", rootnetBaseOpts(true), 0xa871909bd159b647},
	} {
		h := fnv.New64a()
		var buf [8]byte
		put := func(v uint64) {
			binary.LittleEndian.PutUint64(buf[:], v)
			h.Write(buf[:])
		}
		putf := func(f float64) { put(math.Float64bits(f)) }
		sr := NewSearcher(tc.o, &fakeNet{val: -1}, Heuristic{})
		for seed := uint64(1); seed <= 4; seed++ {
			steps, winner := seatGame(sr, seed)
			for i := range steps {
				r := &steps[i].res
				put(uint64(r.N))
				put(uint64(r.Choice))
				put(uint64(r.NActs))
				for j := 0; j < r.N; j++ {
					put(uint64(uint32(r.Visits[j])))
					putf(r.Q[j])
					putf(r.Prior[j])
				}
				putf(r.RootValue)
				put(uint64(uint32(r.Stats.Completed)))
			}
			put(uint64(uint8(winner)))
		}
		got := h.Sum64()
		if got != tc.want {
			t.Errorf("%s: digest %016x, want %016x", tc.name, got, tc.want)
		}
	}
}

// rootnetBaseOpts is the two pinned configurations: a plain tree and the
// fixed-world node cache.
func rootnetBaseOpts(cached bool) Options {
	o := DefaultOptions()
	o.Sims = 32
	if cached {
		o.CachedWorlds = 4
		o.NodeCache = 64
	}
	return o
}

// TestRootEvalRootPrior: the root point's prior is RootEval's, exactly the
// prior a search with RootEval alone would get; every other point keeps
// Eval's.
func TestRootEvalRootPrior(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 16
	rs := roots(20)
	a := &fakeNet{val: 0.2, flat: true}
	b1 := &fakeNet{val: 0.8}
	b2 := &fakeNet{val: 0.8}
	srB := NewSearcher(o, b1, Heuristic{})
	srAB := NewSearcher(o, a, Heuristic{})
	srAB.RootEval = b2
	srA := NewSearcher(o, &fakeNet{val: 0.2, flat: true}, Heuristic{})
	searched, differs := 0, 0
	for i := range rs {
		resB := srB.Search(rs[i], uint64(i))
		resAB := srAB.Search(rs[i], uint64(i))
		resA := srA.Search(rs[i], uint64(i))
		if resAB.N != resB.N {
			t.Fatalf("root %d: %d candidates with RootEval, %d with B alone", i, resAB.N, resB.N)
		}
		for j := 0; j < resB.N; j++ {
			if resAB.Keys[j] != resB.Keys[j] || resAB.Prior[j] != resB.Prior[j] {
				t.Fatalf("root %d cand %d: RootEval prior (%x %v) != B alone (%x %v)",
					i, j, resAB.Keys[j], resAB.Prior[j], resB.Keys[j], resB.Prior[j])
			}
		}
		if resAB.Stats.Searched == 0 {
			continue
		}
		searched++
		if resAB.Stats.RootPrior != 1 {
			t.Fatalf("root %d: RootPrior %d, want 1", i, resAB.Stats.RootPrior)
		}
		if resAB.Prior != resA.Prior {
			differs++
		}
	}
	if searched != len(rs) {
		t.Fatalf("searched %d of %d roots", searched, len(rs))
	}
	if b2.priors != searched {
		t.Fatalf("RootEval Priors calls %d, want %d (one per searched root)", b2.priors, searched)
	}
	if differs == 0 {
		t.Fatalf("RootEval prior never differed from A's: the test is vacuous")
	}
}

// TestRootEvalRootValue: the root node starts at (1, RootEval's value of
// the real root); every leaf is still Eval's.
func TestRootEvalRootValue(t *testing.T) {
	rs := roots(20)
	check := func(t *testing.T, o Options, cached bool) {
		t.Helper()
		for i := range rs {
			with := NewSearcher(o, &fakeNet{val: 0.2}, Heuristic{})
			with.RootEval = &fakeNet{val: 0.8}
			plain := NewSearcher(o, &fakeNet{val: 0.2}, Heuristic{})
			res := with.Search(rs[i], uint64(i))
			base := plain.Search(rs[i], uint64(i))
			if res.Stats.Terminal != 0 {
				continue
			}
			// Evaluator.Value returns float32: the root and leaf values
			// arrive as their float64 widenings.
			aV, bV := float64(float32(0.2)), float64(float32(0.8))
			if cached {
				// a cached tree's RootValue is its first world's root only
				if res.RootValue <= aV+1e-9 {
					t.Fatalf("root %d: cached RootValue %v is not above A's", i, res.RootValue)
				}
			} else {
				k := float64(res.Stats.Completed)
				want := (bV + aV*k) / (1 + k)
				if d := math.Abs(res.RootValue - want); d >= 1e-12 {
					t.Fatalf("root %d: RootValue %v, want %v (k=%v)", i, res.RootValue, want, k)
				}
			}
			if d := math.Abs(base.RootValue - aV); d >= 1e-12 {
				t.Fatalf("root %d: unset RootValue %v, want %v", i, base.RootValue, aV)
			}
		}
	}
	o := DefaultOptions()
	o.Sims = 4
	check(t, o, false)
	o.CachedWorlds = 2
	o.NodeCache = 64
	check(t, o, true)
}

// TestRootEvalDeterministic: RootEval draws no randomness; the same seed
// gives the same games.
func TestRootEvalDeterministic(t *testing.T) {
	mk := func() *Searcher {
		sr := NewSearcher(DefaultOptions(), &fakeNet{val: -1}, Heuristic{})
		sr.RootEval = &fakeNet{val: -1, flat: true}
		return sr
	}
	for seed := uint64(5); seed <= 6; seed++ {
		a, b := mk(), mk()
		stepsA, winA := seatGame(a, seed)
		stepsB, winB := seatGame(b, seed)
		if len(stepsA) != len(stepsB) {
			t.Fatalf("seed %d: %d steps vs %d", seed, len(stepsA), len(stepsB))
		}
		for i := range stepsA {
			if !sameSearch(&stepsA[i].res, &stepsB[i].res) {
				t.Fatalf("seed %d step %d: searches differ", seed, i)
			}
		}
		if winA != winB {
			t.Fatalf("seed %d: winners %d vs %d", seed, winA, winB)
		}
	}
}

// TestRootEvalZeroAllocs: the root hook allocates nothing.
func TestRootEvalZeroAllocs(t *testing.T) {
	sr := NewSearcher(DefaultOptions(), &fakeNet{val: 0.5}, Heuristic{})
	sr.RootEval = &fakeNet{val: 0.6}
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 11, dirac.Config{})
	pol := dirac.NewRandomPolicy(5)
	var d dirac.Decision
	for i := 0; i < 60; i++ { // into the game
		s.StepInto(&d)
		s.Apply(pol.Pick(&d))
	}
	for {
		s.StepInto(&d)
		if d.Player == 0 && d.Kind == dirac.DkPriority {
			break
		}
		s.Apply(pol.Pick(&d))
	}
	allocs := testing.AllocsPerRun(5, func() { sr.Search(&s, 1) })
	if allocs != 0 {
		t.Fatalf("allocs per search = %v", allocs)
	}
}

// TestRootEvalNeedsNetPrior: a RootEval paired with a prior-less Eval is
// ignored.
func TestRootEvalNeedsNetPrior(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 16
	rs := roots(10)
	b := &fakeNet{val: 0.9}
	with := NewSearcher(o, Uniform{}, Heuristic{})
	with.RootEval = b
	plain := NewSearcher(o, Uniform{}, Heuristic{})
	for i := range rs {
		ra := with.Search(rs[i], uint64(i))
		rb := plain.Search(rs[i], uint64(i))
		if ra.Stats.RootPrior != 0 {
			t.Fatalf("root %d: RootPrior %d with a prior-less Eval", i, ra.Stats.RootPrior)
		}
		if !sameSearch(ra, rb) {
			t.Fatalf("root %d: RootEval changed a search whose Eval has no prior", i)
		}
	}
	if b.priors != 0 {
		t.Fatalf("RootEval Priors calls %d, want 0", b.priors)
	}
}
