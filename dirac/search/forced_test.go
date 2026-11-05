package search

import (
	"math"
	"testing"
)

// TestForcedOffIsPlain: ForcedK 0 is the plain search (no target, no
// counters).
func TestForcedOffIsPlain(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 30
	a := NewSearcher(o, Uniform{}, Heuristic{})
	b := NewSearcher(o, Uniform{}, Heuristic{})
	rs := roots(16)
	for i := range rs {
		x, y := *a.Search(rs[i], uint64(i)), *b.Search(rs[i], uint64(i))
		if !sameSearch(&x, &y) || x.HasTarget || x.Stats.ForcedSearched != 0 || x.Stats.ForcedPlayouts != 0 {
			t.Fatalf("root %d: ForcedK 0 is not the plain search", i)
		}
	}
}

// TestForcedPlayoutsAndPruning mirrors azmcts TestForcedPlayouts: under
// ForcedK every root child the search reaches gets close to its forced
// visits, the most-visited child keeps its visits in Target, no child gains,
// a child left with one visit or none is dropped, the argmax move is
// unchanged, and the counters move; the node cache stays exact under it.
func TestForcedPlayoutsAndPruning(t *testing.T) {
	o := DefaultOptions()
	o.Sims, o.ForcedK = 60, 2
	net := &fakeNet{val: -1}
	sr := NewSearcher(o, net, Heuristic{})
	rs := roots(24)
	forced, pruned := 0, 0
	for i := range rs {
		r := *sr.Search(rs[i], uint64(i)+3)
		if r.Stats.Searched == 0 {
			continue
		}
		if !r.HasTarget || r.Stats.ForcedSearched != 1 {
			t.Fatalf("root %d: no target under ForcedK", i)
		}
		best, total := 0, int32(0)
		for c := 0; c < r.N; c++ {
			total += r.Visits[c]
			if r.Visits[c] > r.Visits[best] {
				best = c
			}
		}
		if r.Choice != best || r.Target[best] != r.Visits[best] {
			t.Fatalf("root %d: choice %d (argmax %d), target %d of %d", i, r.Choice, best, r.Target[best], r.Visits[best])
		}
		for c := 0; c < r.N; c++ {
			if r.Target[c] > r.Visits[c] || r.Target[c] == 1 {
				t.Fatalf("root %d child %d: target %d from %d visits", i, c, r.Target[c], r.Visits[c])
			}
			// every child the root offered in every simulation holds about
			// its forced visits
			// (lagging by at most one visit per child: one forced child is
			// served per simulation)
			if r.Avail[c] == int32(r.Stats.Simulations) && float64(r.Visits[c]+int32(r.N)) < math.Floor(forcedVisits(2, r.Prior[c], total)) {
				t.Fatalf("root %d child %d: %d visits below forced %g", i, c, r.Visits[c], forcedVisits(2, r.Prior[c], total))
			}
		}
		forced += r.Stats.ForcedPlayouts
		pruned += r.Stats.PrunedVisits
	}
	if forced == 0 || pruned == 0 {
		t.Fatalf("forced playouts %d, pruned visits %d: the rule never acted", forced, pruned)
	}

	// exact under the node cache
	o.CachedWorlds = 4
	plain := NewSearcher(o, net, Heuristic{})
	o.NodeCache = 64
	cached := NewSearcher(o, net, Heuristic{})
	for i := range rs {
		a, b := *plain.Search(rs[i], uint64(i)), *cached.Search(rs[i], uint64(i))
		if !sameSearch(&a, &b) || a.Stats.ForcedPlayouts != b.Stats.ForcedPlayouts {
			t.Fatalf("root %d: the node cache changed a forced search (playouts %d vs %d)", i, a.Stats.ForcedPlayouts, b.Stats.ForcedPlayouts)
		}
	}

	for _, k := range []float64{-1, math.NaN(), math.Inf(1)} {
		bad := DefaultOptions()
		bad.ForcedK = k
		if bad.Validate() == nil {
			t.Fatalf("ForcedK %g validated", k)
		}
	}
}

// TestPruneTarget checks pruneTarget's arithmetic on a hand-made root
// (azmcts TestPruneTarget's shape; cpuct 1.5, 73 visits): the best child
// keeps its 50; a low-value low-prior child's 3 fall to one and are dropped;
// a child whose PUCT value with one visit fewer (0.784) still beats the
// best's (0.727) keeps its 20.
func TestPruneTarget(t *testing.T) {
	var sr Searcher
	sr.Opts = DefaultOptions()
	sr.Opts.ForcedK = 2
	var r Result
	r.N = 3
	r.Visits = [MaxCands]int32{50, 3, 20}
	r.Q = [MaxCands]float64{0.6, 0.1, 0.59}
	r.Avail = [MaxCands]int32{73, 73, 73}
	sr.rootPt.prior = [MaxCands]float64{0.5, 0.05, 0.3}
	sr.pruneTarget(&r)
	if r.Target[0] != 50 {
		t.Fatalf("best child target %d", r.Target[0])
	}
	if r.Target[1] != 0 {
		t.Fatalf("a poor child kept %d visits", r.Target[1])
	}
	if r.Target[2] != 20 {
		t.Fatalf("a close child target %d of 20", r.Target[2])
	}
	if sr.St.PrunedChildren != 1 || sr.St.PrunedVisits != 3 {
		t.Fatalf("counters %+v", sr.St)
	}
}
