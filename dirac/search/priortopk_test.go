package search

import "testing"

// TestPriorRankMaxOptionsSkipsTheRanking mirrors azmcts's: a cap no point
// reaches is byte-identical to the plain top-k search; a cap below every
// decision's option count never calls the network's Priors, keeps the
// enumeration's first k under a uniform prior (the same candidates and
// prior as a network whose logits are all equal) and counts a fallback at
// every point; a negative cap is refused.
func TestPriorRankMaxOptionsSkipsTheRanking(t *testing.T) {
	o := DefaultOptions()
	o.Sims, o.PriorTopK = 40, 3
	rs := roots(24)
	run := func(net *fakeNet, rankMax int) []Result {
		oo := o
		oo.PriorRankMaxOptions = rankMax
		sr := NewSearcher(oo, net, Heuristic{})
		out := make([]Result, len(rs))
		for i := range rs {
			out[i] = *sr.Search(rs[i], uint64(i)+5)
		}
		return out
	}
	plain := run(&fakeNet{val: 0.6}, 0)
	high := run(&fakeNet{val: 0.6}, 10000)
	cut := 0
	for i := range plain {
		if !sameSearch(&plain[i], &high[i]) || high[i].Stats.PriorFallbacks != 0 {
			t.Fatalf("root %d: a cap no point reaches changed the search", i)
		}
		if plain[i].Stats.Searched > 0 && plain[i].N == 3 {
			cut++
		}
	}
	if cut == 0 {
		t.Fatalf("no root was cut to the top 3: the test proves nothing")
	}
	skipNet := &fakeNet{val: 0.6}
	skipped := run(skipNet, 1)
	flat := run(&fakeNet{val: 0.6, flat: true}, 0)
	if skipNet.priors != 0 {
		t.Fatalf("rankMax 1: Priors called %d times", skipNet.priors)
	}
	differ := 0
	for i := range skipped {
		a, b := &skipped[i], &flat[i]
		if a.N != b.N || a.Keys != b.Keys || a.Prior != b.Prior {
			t.Fatalf("root %d: the skipped ranking is not the uniform top-k cut: keys %v vs %v, prior %v vs %v",
				i, a.Keys[:a.N], b.Keys[:b.N], a.Prior[:a.N], b.Prior[:b.N])
		}
		if a.Stats.Searched > 0 && a.Stats.PriorFallbacks == 0 {
			t.Fatalf("root %d: no fallback counted", i)
		}
		if a.Keys != plain[i].Keys || a.Prior != plain[i].Prior {
			differ++
		}
	}
	if differ == 0 {
		t.Fatalf("the skipped ranking never changed a root's candidates or prior")
	}
	bad := DefaultOptions()
	bad.PriorRankMaxOptions = -1
	if bad.Validate() == nil {
		t.Fatalf("a negative cap validated")
	}
}
