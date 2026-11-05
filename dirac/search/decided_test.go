package search

import "testing"

// TestDecidedRun mirrors azmcts's decidedState tests: a value above the
// band extends a winning run, one below starts a losing one, one inside
// ends it.
func TestDecidedRun(t *testing.T) {
	var st SeatState
	for i, c := range []struct {
		v    float64
		side int8
		run  int
	}{
		{0.95, 1, 1}, {0.97, 1, 2}, {0.5, 0, 0}, {0.02, -1, 1}, {0.05, -1, 2}, {0.99, 1, 1}, {0.9, 0, 0},
	} {
		st.observe(c.v, 0.9)
		if st.side != c.side || st.run != c.run {
			t.Fatalf("step %d (v %g): side %d run %d, want %d %d", i, c.v, st.side, st.run, c.side, c.run)
		}
	}
}

// TestDecidedOffLeavesTheGameUnchanged mirrors azmcts's: DecidedHigh 0 is
// the plain seat whatever the other three say.
func TestDecidedOffLeavesTheGameUnchanged(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 12
	plain, _ := seatGame(NewSearcher(o, &fakeNet{val: 0.97}, Heuristic{}), 5)
	o.DecidedN, o.DecidedSims, o.DecidedWeight = 1, 2, 0.1
	got, _ := seatGame(NewSearcher(o, &fakeNet{val: 0.97}, Heuristic{}), 5)
	if len(got) != len(plain) {
		t.Fatalf("%d decisions, plain %d", len(got), len(plain))
	}
	for i := range got {
		if !sameSearch(&got[i].res, &plain[i].res) || got[i].res.Decided || got[i].res.Weight != 1 {
			t.Fatalf("decision %d: DecidedHigh 0 changed the seat", i)
		}
	}
}

// TestDecidedSearches mirrors azmcts TestDecidedSearchesAndWeights: with a
// leaf of 0.97 every completed search's root value is above 0.9, so after
// DecidedN of them each later full search is decided -- DecidedSims, no
// noise, no forced playouts, recorded at DecidedWeight (not at all at 0) --
// and the counts add up; a playout-cap cheap search stays cheap.
func TestDecidedSearches(t *testing.T) {
	o := DefaultOptions()
	o.Sims, o.ForcedK = 12, 2
	o.DecidedHigh, o.DecidedN, o.DecidedSims, o.DecidedWeight = 0.9, 2, 3, 0.25
	sr := NewSearcher(o, &fakeNet{val: 0.97}, Heuristic{})
	var st SeatState
	steps, _ := seatGameState(sr, 6, &st)
	var shadow SeatState // the run as the test predicts it
	decided, winning := 0, 0
	for i, x := range steps {
		r := &x.res
		if r.Stats.Searched == 0 {
			continue
		}
		want := shadow.side != 0 && shadow.run >= 2
		if r.Decided != want {
			t.Fatalf("decision %d (run %d side %d): decided %v", i, shadow.run, shadow.side, r.Decided)
		}
		if r.Decided {
			decided++
			if shadow.side > 0 {
				winning++
			}
			if r.Stats.Simulations != 3 || r.HasTarget || r.Weight != 0.25 || !r.Record {
				t.Fatalf("decision %d: a decided search: %d sims, target %v, weight %g, record %v", i, r.Stats.Simulations, r.HasTarget, r.Weight, r.Record)
			}
		} else if r.Stats.Simulations != 12 || !r.HasTarget || r.Weight != 1 {
			t.Fatalf("decision %d: a full search: %d sims, target %v, weight %g", i, r.Stats.Simulations, r.HasTarget, r.Weight)
		}
		if r.Stats.Completed > 0 {
			shadow.observe(r.RootValue, 0.9)
		}
	}
	if decided == 0 || st.Decided != decided || st.Winning != winning || winning == 0 {
		t.Fatalf("decided %d (winning %d), seat counts %d (winning %d)", decided, winning, st.Decided, st.Winning)
	}

	// weight 0: decided searches are never recorded
	o.DecidedWeight = 0
	steps, _ = seatGame(NewSearcher(o, &fakeNet{val: 0.97}, Heuristic{}), 6)
	n := 0
	for _, x := range steps {
		if x.res.Decided {
			n++
			if x.res.Record {
				t.Fatalf("a weight-0 decided search is recorded")
			}
		}
	}
	if n == 0 {
		t.Fatalf("no decided search at weight 0")
	}

	// with playout-cap randomisation a cheap search stays cheap
	o.DecidedWeight, o.FullProb, o.CheapSims = 0.25, 0.5, 2
	steps, _ = seatGame(NewSearcher(o, &fakeNet{val: 0.97}, Heuristic{}), 6)
	sawCheap, sawDecided := false, false
	for _, x := range steps {
		if x.res.Cheap && x.res.Decided {
			t.Fatalf("a search both cheap and decided")
		}
		sawCheap = sawCheap || x.res.Cheap
		sawDecided = sawDecided || x.res.Decided
	}
	if !sawCheap || !sawDecided {
		t.Fatalf("cheap %v decided %v: the mix never arose", sawCheap, sawDecided)
	}

	for _, mut := range []func(*Options){
		func(o *Options) { o.DecidedHigh = 0.5 },
		func(o *Options) { o.DecidedHigh = 1 },
		func(o *Options) { o.DecidedN = 0 },
		func(o *Options) { o.DecidedSims = 0 },
		func(o *Options) { o.DecidedWeight = 1.5 },
	} {
		oo := DefaultOptions()
		oo.DecidedHigh, oo.DecidedN, oo.DecidedSims, oo.DecidedWeight = 0.9, 2, 3, 0.25
		if oo.Validate() != nil {
			t.Fatalf("the base decided options do not validate")
		}
		mut(&oo)
		if oo.Validate() == nil {
			t.Fatalf("%+v validated", oo)
		}
	}
}
