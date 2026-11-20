package dirac

import (
	"runtime"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The tapsdefer fixture's decks (dirac/oracle/taps_planner_defer_test.go's):
// Judge of Currents defers its Merfolk {T} dorks, so a play the printed
// sources cannot pay searches the gaps, and a board whose only census gaps
// are those sources searches their subsets (deferGapSearch).
var tapsDeferGapDecks = [2][]cardset.Entry{
	{
		{Name: "Judge of Currents", Count: 4}, {Name: "Firdoch Core", Count: 3}, {Name: "Birds of Paradise", Count: 4},
		{Name: "Savannah Lions", Count: 5}, {Name: "Hawkeye's Bow", Count: 2},
		{Name: "Plains", Count: 22},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 5}, {Name: "Lightning Bolt", Count: 4},
		{Name: "Shock", Count: 3}, {Name: "Plains", Count: 11}, {Name: "Mountain", Count: 11},
	},
}

// TestTapsDeferGapSearchAllocatesNothing plays the tapsdefer fixture with the
// random policy until a seed's decision reaches the Taps-deferred gaps'
// subsets search (deferGapSearch, TapsDeferGapStats), then pins that
// decision's allocation count at zero on a copy of the state. The search's
// locals were a map and three slices (the sweep's allocation): the transition
// may allocate nothing.
func TestTapsDeferGapSearchAllocatesNothing(t *testing.T) {
	t.Parallel()
	set := cardset.AllowUnverified(cardsettest.Set(t, "tapsdefer"))
	tbl, ids, err := set.MatchupN(tapsDeferGapDecks[:2])
	if err != nil {
		t.Fatalf("tapsdefer: %v", err)
	}
	t.Cleanup(func() { ReleaseTable(tbl) })
	Register(tbl)
	var dk [MaxSeats][]CardID
	copy(dk[:], ids)
	var s State
	var d Decision
	found := false
	for seed := uint64(1); seed <= 40 && !found; seed++ {
		s.InitN(tbl, dk, 2, seed, Config{Mulligans: 2})
		pol := NewRandomPolicy(seed)
		func() {
			defer func() {
				if recover() != nil {
					t.Logf("tapsdefer seed %d: ended on a panic", seed)
				}
			}()
			for i := 0; i < 4000 && !found; i++ {
				before := TapsDeferGapStats.Searches.Load()
				s.StepInto(&d)
				reached := TapsDeferGapStats.Searches.Load() > before
				if d.Kind == DkNone {
					return
				}
				if reached {
					found = true
					// The segment: on a copy, play forward until the subsets
					// search runs again, and read the whole segment's heap
					// allocations. The transition allocates nothing (the
					// hot-path invariant), so every object is the search's.
					// A pinned-decision replay cannot do this: StepInto's
					// verdict is cached in the state, so a second StepInto of
					// the same decision walks nothing.
					var snap State
					snap.CopyFrom(&s)
					// CopyFrom keeps dst's own fork pool, so the copy's first
					// fork would allocate the pool and its State: give the
					// segment a pooled one, warmed past the subsets search's
					// nesting (each level holds its replay's and chain's
					// probes, so the walk's depth is past the gap count) so no
					// fork in the segment allocates its State.
					snap.UseForkPool(NewForkPool())
					for d := 0; d < 64; d++ {
						snap.fork()
					}
					snap.pool.depth = 0
					oldProcs := runtime.GOMAXPROCS(1)
					defer runtime.GOMAXPROCS(oldProcs)
					runtime.GC()
					var m0, m1 runtime.MemStats
					runtime.ReadMemStats(&m0)
					base := TapsDeferGapStats.Searches.Load()
					pol2 := NewRandomPolicy(seed)
					var d2 Decision
					steps := 0
					func() {
						defer func() { recover() }()
						for i := 0; i < 2000; i++ {
							snap.StepInto(&d2)
							if d2.Kind == DkNone {
								return
							}
							snap.Apply(pol2.Pick(&d2))
							steps++
							if TapsDeferGapStats.Searches.Load() > base {
								return
							}
						}
					}()
					runtime.ReadMemStats(&m1)
					if TapsDeferGapStats.Searches.Load() == base {
						t.Fatal("the 2000-step segment from the pinned state never reaches deferGapSearch again: the allocation cannot be measured")
					}
					if n := m1.Mallocs - m0.Mallocs; n != 0 {
						t.Fatalf("the %d-step segment reaching deferGapSearch allocated %d objects", steps, n)
					}
					t.Logf("tapsdefer seed %d decision %d: the %d-step segment reaches deferGapSearch with 0 allocs", seed, i, steps)
				}
				s.Apply(pol.Pick(&d))
			}
		}()
	}
	if !found {
		t.Fatal("no seed of the tapsdefer fixture reached deferGapSearch: the search's allocation cannot be measured")
	}
}
