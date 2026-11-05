package search

import "testing"

// TestSimsByKindSetsTheBudgetPerKind mirrors azmcts's: over a played game,
// a priority search runs SimsByKind's 2 simulations and every other kind
// Sims' 4; with no entry set SeatSearch is Search.
func TestSimsByKindSetsTheBudgetPerKind(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 4
	o.SimsByKind[kindPriority] = 2
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	byKind := map[bool]int{}
	for seed := uint64(1); seed <= 3; seed++ {
		steps, _ := seatGame(sr, seed)
		for _, st := range steps {
			r := &st.res
			if r.Stats.Searched == 0 {
				continue
			}
			want := 4
			prio := r.Kind == kindPriority
			if prio {
				want = 2
			}
			if r.Stats.Simulations != want {
				t.Fatalf("seed %d: a %v search ran %d simulations, want %d", seed, prio, r.Stats.Simulations, want)
			}
			byKind[prio]++
		}
	}
	if byKind[true] == 0 || byKind[false] == 0 {
		t.Fatalf("searched priority %d, other %d: the game never reached both", byKind[true], byKind[false])
	}
	if sr.Opts.Sims != 4 {
		t.Fatalf("SeatSearch left Sims at %d", sr.Opts.Sims)
	}
	plain := DefaultOptions()
	plain.Sims = 4
	a, _ := seatGame(NewSearcher(plain, Uniform{}, Heuristic{}), 2)
	plain.SimsByKind[kindPriority] = 4 // the same budget: the same game
	b, _ := seatGame(NewSearcher(plain, Uniform{}, Heuristic{}), 2)
	if len(a) != len(b) {
		t.Fatalf("SimsByKind at Sims changed the game: %d vs %d decisions", len(a), len(b))
	}
	for i := range a {
		if !sameSearch(&a[i].res, &b[i].res) {
			t.Fatalf("decision %d: SimsByKind at Sims changed the search", i)
		}
	}
	bad := DefaultOptions()
	bad.SimsByKind[kindBlockers] = -1
	if bad.Validate() == nil {
		t.Fatalf("a negative SimsByKind entry validated")
	}
}
