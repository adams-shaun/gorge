package search

import "testing"

// TestLazyPriorsIsExact: LazyPriors skips only points whose priors nothing
// reads, so the search is byte-identical with it on and off (Result and
// every Stats counter but PriorSkips) and the network's Priors calls drop.
// The cases reach the skip rule's edges: a cap above and below Limit (no
// cut vs frequent cuts), opponent points without a top-k cut, and the
// forced-playout root path.
func TestLazyPriorsIsExact(t *testing.T) {
	cases := []struct {
		name  string
		tweak func(*Options)
	}{
		{"default", func(*Options) {}},
		{"topk8", func(o *Options) { o.PriorTopK = 8 }},
		{"topk2", func(o *Options) { o.PriorTopK = 2 }},
		{"nooppmirror", func(o *Options) { o.OpponentMirror = false }},
		{"forced", func(o *Options) { o.ForcedK = 2 }},
	}
	rs := roots(40)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eo, lo := DefaultOptions(), DefaultOptions()
			c.tweak(&eo)
			c.tweak(&lo)
			eo.LazyPriors, lo.LazyPriors = false, true
			en, ln := &fakeNet{val: -1}, &fakeNet{val: -1}
			esr, lsr := NewSearcher(eo, en, Heuristic{}), NewSearcher(lo, ln, Heuristic{})
			skips := 0
			for i := range rs {
				a := esr.Search(rs[i], uint64(i))
				b := lsr.Search(rs[i], uint64(i))
				if !sameSearch(a, b) {
					t.Fatalf("root %d: LazyPriors changed the search", i)
				}
				bs := b.Stats
				skips += bs.PriorSkips
				bs.PriorSkips = 0
				if a.Stats != bs {
					t.Fatalf("root %d: stats differ:\neager %+v\nlazy  %+v", i, a.Stats, bs)
				}
			}
			if skips == 0 {
				t.Fatalf("no point skipped the ranking: the rule never acted")
			}
			if ln.priors >= en.priors {
				t.Fatalf("Priors calls lazy %d, eager %d: the skip saved nothing", ln.priors, en.priors)
			}
			t.Logf("%s: %d skips, eager %d Priors, lazy %d", c.name, skips, en.priors, ln.priors)
		})
	}
}

// TestLazyPriorsGameUnchanged: a whole seat-search game is the same with
// the knob on and off (same winner, same decisions, same results).
func TestLazyPriorsGameUnchanged(t *testing.T) {
	for seed := uint64(1); seed <= 3; seed++ {
		eo, lo := DefaultOptions(), DefaultOptions()
		eo.Sims, lo.Sims = 50, 50
		eo.LazyPriors, lo.LazyPriors = false, true
		eager, wE := seatGame(NewSearcher(eo, &fakeNet{val: -1}, Heuristic{}), seed)
		lazy, wL := seatGame(NewSearcher(lo, &fakeNet{val: -1}, Heuristic{}), seed)
		if wE != wL {
			t.Fatalf("seed %d: winner %d then %d", seed, wE, wL)
		}
		if len(eager) != len(lazy) {
			t.Fatalf("seed %d: %d decisions then %d", seed, len(eager), len(lazy))
		}
		for i := range eager {
			if !sameSearch(&eager[i].res, &lazy[i].res) {
				t.Fatalf("seed %d decision %d: LazyPriors changed the search", seed, i)
			}
		}
	}
}
