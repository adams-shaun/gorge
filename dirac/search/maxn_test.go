package search

import "testing"

// TestMaxNTwoSeatsTakesOldPath: with two living seats max^n and paranoid
// coincide, so MaxN on runs the scalar path: every search of a two-seat game
// returns the MaxN-off result and counters, under a VecEvaluator leaf and a
// split one.
func TestMaxNTwoSeatsTakesOldPath(t *testing.T) {
	for _, ev := range []func() Evaluator{
		func() Evaluator { return HeuristicLeaf{} },
		func() Evaluator { return &fakeNet{val: -1} },
	} {
		off := rootnetBaseOpts(false)
		on := off
		on.MaxN = true
		srOff := NewSearcher(off, ev(), Heuristic{})
		srOn := NewSearcher(on, ev(), Heuristic{})
		opp := 0
		for seed := uint64(1); seed <= 3; seed++ {
			a, wa := seatGame(srOff, seed)
			b, wb := seatGame(srOn, seed)
			if len(a) != len(b) || wa != wb {
				t.Fatalf("%T seed %d: %d searches won by %d, MaxN on %d won by %d", ev(), seed, len(a), wa, len(b), wb)
			}
			for i := range a {
				ra, rb := &a[i].res, &b[i].res
				if !sameSearch(ra, rb) || ra.Stats != rb.Stats {
					t.Fatalf("%T seed %d search %d: MaxN on differs", ev(), seed, i)
				}
				if rb.Stats.MaxN != 0 {
					t.Fatalf("%T seed %d search %d: max^n ran with two seats", ev(), seed, i)
				}
				opp += ra.Stats.OppPoints
			}
		}
		if opp == 0 {
			t.Fatalf("%T: precondition: no opponent point was selected at", ev())
		}
	}
}

// TestMaxNOpponentMaximisesOwnComponent: at an opponent's point with both
// replies visited equally, paranoid selection takes the reply worst for the
// searching seat (y) and max^n the reply best for the acting opponent's own
// entry (x).
func TestMaxNOpponentMaximisesOwnComponent(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 4
	o.MaxN = true
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	tr := &sr.t
	tr.reset()
	var pt point
	pt.opp, pt.seat = true, 1
	pt.addSeq([]uint32{1})
	pt.addSeq([]uint32{2})
	pt.keys[0], pt.keys[1] = 1, 2
	pt.prior[0], pt.prior[1] = 0.5, 0.5
	ni := tr.newNode(&pt)
	vecs := [2][2]float64{{0.5, 0.4}, {0.2, 0.1}} // x, y: entries 0 (the searching seat) and 1 (the opponent)
	nd := &tr.nodes[ni]
	i := 0
	for e := nd.first; e >= 0; e = tr.edges[e].sib {
		kid := &tr.edges[e]
		kid.n, kid.avail, kid.w = 10, 10, 10*vecs[i][0]
		nd.n += 10
		nd.w += kid.w
		for r := range vecs[i] {
			tr.ewv[e][r] = 10 * vecs[i][r]
			tr.nwv[ni][r] += 10 * vecs[i][r]
		}
		i++
	}
	if !(vecs[0][1] > vecs[1][1] && vecs[1][0] < vecs[0][0]) {
		t.Fatal("precondition: the opponent's own best reply must differ from the searching seat's worst")
	}
	tr.vec = false
	if got := tr.edges[tr.selectEdge(ni, &pt, &o)].key; got != 2 {
		t.Fatalf("paranoid reply %d, want y (2)", got)
	}
	tr.vec = true
	if got := tr.edges[tr.selectEdge(ni, &pt, &o)].key; got != 1 {
		t.Fatalf("max^n reply %d, want x (1)", got)
	}
	pt.opp = false // the searching seat's own point reads the scalar
	if got := tr.edges[tr.selectEdge(ni, &pt, &o)].key; got != 1 {
		t.Fatalf("own reply %d, want x (1)", got)
	}
}

// TestMaxNValidate: MaxN does not combine with the node cache or the
// batched path, whose backups carry the scalar alone.
func TestMaxNValidate(t *testing.T) {
	o := DefaultOptions()
	o.MaxN = true
	if err := o.Validate(); err != nil {
		t.Fatalf("MaxN alone: %v", err)
	}
	for _, f := range []func(*Options){
		func(o *Options) { o.CachedWorlds, o.NodeCache = 4, 64 },
		func(o *Options) { o.Batch = 8 },
	} {
		x := o
		f(&x)
		if x.Validate() == nil {
			t.Fatalf("MaxN validated with NodeCache %d Batch %d", x.NodeCache, x.Batch)
		}
	}
}
