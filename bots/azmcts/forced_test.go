package azmcts

import (
	"context"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// sharpGame is a root whose low-prior child b is a little worse than a:
// plain PUCT at a small cpuct starves b, forced playouts keep visiting it.
func sharpGame() *fakeGame {
	return &fakeGame{nodes: map[string]fakeNode{
		"":   {keys: []Key{"a", "b", "c"}, prior: []float64{0.85, 0.1, 0.05}, value: 0.5},
		"/a": {value: 0.6, terminal: true},
		"/b": {value: 0.5, terminal: true},
		"/c": {value: 0.2, terminal: true},
	}}
}

// Under ForcedK every root child reaches its forced visits (up to the one
// simulation the last increase of the total may still owe it), which plain
// PUCT at cpuct 0.25 never gives the low-prior children; the forced
// selections that overrode PUCT are counted, and in-walk points are never
// forced (a one-level tree: nothing below the root to force).
func TestForcedPlayoutsAtTheRoot(t *testing.T) {
	const sims = 200
	run := func(k float64) (TreeResult, Stats) {
		t.Helper()
		g := sharpGame()
		o := treeOpts(sims)
		o.CPUCT, o.ForcedK = 0.25, k
		var st Stats
		tr, err := RunTree(context.Background(), rootOf(g), &fakeSource{g: g}, o, &st)
		if err != nil {
			t.Fatal(err)
		}
		return tr, st
	}
	plain, pst := run(0)
	forced, fst := run(2)
	if pst.ForcedPlayouts != 0 || fst.ForcedPlayouts == 0 {
		t.Fatalf("forced playouts counted: plain %d, forced %d", pst.ForcedPlayouts, fst.ForcedPlayouts)
	}
	total := 0
	for _, v := range forced.Visits {
		total += v
	}
	prior := sharpGame().nodes[""].prior
	for i, v := range forced.Visits {
		if nf := forcedVisits(2, prior[i], total); float64(v) < nf-1 {
			t.Fatalf("child %d: %d visits, below its %.2f forced", i, v, nf)
		}
	}
	if forced.Visits[1] <= plain.Visits[1] || forced.Visits[2] <= plain.Visits[2] {
		t.Fatalf("forced visits %v, plain %v: the low-prior children gained nothing", forced.Visits, plain.Visits)
	}
	t.Logf("plain %v, forced %v (%d overrides)", plain.Visits, forced.Visits, fst.ForcedPlayouts)
}

// With ForcedK 0 the root selection is selectEdge's, exactly: the same
// visits, values and counters as a tree run before the option existed.
func TestForcedOffIsPlainPUCT(t *testing.T) {
	g := sharpGame()
	o := treeOpts(50)
	o.CPUCT = 0.25
	var st Stats
	tr, err := RunTree(context.Background(), rootOf(g), &fakeSource{g: g}, o, &st)
	if err != nil {
		t.Fatal(err)
	}
	nd := newNode(rootOf(g))
	nd.n, nd.w = 1, 0.5
	val := map[Key]float64{"a": 0.6, "b": 0.5, "c": 0.2}
	for i := 0; i < 50; i++ {
		e := selectEdge(nd, rootOf(g), o)
		for _, k := range nd.kids {
			k.avail++
		}
		e.n++
		e.w += val[e.key]
		nd.n++
		nd.w += val[e.key]
	}
	for i, k := range nd.kids {
		if k.n != tr.Visits[i] {
			t.Fatalf("child %d: RunTree %d visits, a by-hand selectEdge loop %d", i, tr.Visits[i], k.n)
		}
	}
}

// pruneTarget, by hand (cpuct 0.25, k 2, 100 visits, N_avail 101):
//   - a child bound by PUCT: [90 10], prior .5/.5, Q .6/.2. c*'s PUCT is
//     .6 + .125*sqrt(101)/91 = .61381; b may drop to m while
//     .2 + .125*sqrt(101)/(1+m) < .61381, i.e. m >= 3 (n_forced 10 allows
//     all ten): target [90 3];
//   - a child bound by n_forced: prior .9/.1, n_forced(b) = sqrt(20) =
//     4.47, so b keeps ceil(10-4.47) = 6 (its PUCT stays far below): [90 6];
//   - a child left with one visit is dropped: [99 1] -> [99 0];
//   - a child whose PUCT already exceeds c*'s keeps every visit: [60 40]
//     with Q .5/.55;
//   - ties: c* is the lower index (b, prior .4, keeps ceil(50 - sqrt(80))
//     = 42), and an unvisited child stays at 0.
func TestPruneTarget(t *testing.T) {
	o := DefaultOptions()
	o.CPUCT, o.ForcedK = 0.25, 2
	for _, tc := range []struct {
		name            string
		visits          []int
		q, prior        []float64
		want            []int
		pruned, dropped int
	}{
		{"puct bound", []int{90, 10}, []float64{0.6, 0.2}, []float64{0.5, 0.5}, []int{90, 3}, 7, 0},
		{"forced bound", []int{90, 10}, []float64{0.6, 0.2}, []float64{0.9, 0.1}, []int{90, 6}, 4, 0},
		{"one visit", []int{99, 1}, []float64{0.6, 0.2}, []float64{0.5, 0.5}, []int{99, 0}, 1, 1},
		{"better Q", []int{60, 40}, []float64{0.5, 0.55}, []float64{0.5, 0.5}, []int{60, 40}, 0, 0},
		{"tie and unvisited", []int{50, 50, 0}, []float64{0.6, 0.2, 0}, []float64{0.4, 0.4, 0.2}, []int{50, 42, 0}, 8, 0},
		{"best last", []int{10, 90}, []float64{0.2, 0.6}, []float64{0.1, 0.9}, []int{6, 90}, 4, 0},
	} {
		avail := make([]int, len(tc.visits))
		for i := range avail {
			avail[i] = 100
		}
		var st Stats
		in := append([]int(nil), tc.visits...)
		got := pruneTarget(tc.visits, tc.q, avail, tc.prior, o, &st)
		if !reflect.DeepEqual(got, tc.want) || st.PrunedVisits != tc.pruned || st.PrunedChildren != tc.dropped {
			t.Fatalf("%s: target %v (pruned %d, dropped %d), want %v (%d, %d)", tc.name, got, st.PrunedVisits, st.PrunedChildren, tc.want, tc.pruned, tc.dropped)
		}
		if !reflect.DeepEqual(tc.visits, in) {
			t.Fatalf("%s: pruneTarget changed its input to %v", tc.name, tc.visits)
		}
		if choose(got, false, nil) != choose(tc.visits, false, nil) {
			t.Fatalf("%s: the pruned argmax %d is not the visits' %d", tc.name, choose(got, false, nil), choose(tc.visits, false, nil))
		}
	}
	if got := pruneTarget([]int{0, 0}, []float64{0, 0}, []int{0, 0}, []float64{0.5, 0.5}, o, &Stats{}); !reflect.DeepEqual(got, []int{0, 0}) {
		t.Fatalf("no visits: %v", got)
	}
}

// A real search under ForcedK: Target is the pruned visits (never more than
// Visits per child, c* untouched, no child left at one visit), the argmax
// move is unchanged, and ForcedK 0 has no Target and the same Result as a
// search without the field.
func TestSearchForcedTarget(t *testing.T) {
	cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", testSeed)
	e, d, bot := botPosition(t, cfg, decision.KPriority, 3, 4000)
	o := DefaultOptions()
	o.Sims, o.CPUCT, o.Seed = 60, 0.25, 7
	plain := searchAt(t, e, d, bot, nil, o)
	if plain.Target != nil || plain.Stats.ForcedSearched != 0 {
		t.Fatalf("ForcedK 0: target %v, forced searched %d", plain.Target, plain.Stats.ForcedSearched)
	}
	o.ForcedK = 2
	res := searchAt(t, e, d, bot, nil, o)
	checkResult(t, d, bot, res, 60)
	if res.Stats.ForcedSearched != 1 || len(res.Target) != len(res.Visits) {
		t.Fatalf("forced searched %d, target %v for visits %v", res.Stats.ForcedSearched, res.Target, res.Visits)
	}
	best, pruned := choose(res.Visits, false, nil), 0
	for i, v := range res.Target {
		switch {
		case v > res.Visits[i] || v == 1:
			t.Fatalf("child %d: target %d of %d visits", i, v, res.Visits[i])
		case i == best && v != res.Visits[i]:
			t.Fatalf("the most-visited child lost visits: %d of %d", v, res.Visits[i])
		}
		pruned += res.Visits[i] - v
	}
	if pruned != res.Stats.PrunedVisits || res.Choice != best {
		t.Fatalf("pruned %d (stats %d), choice %d, argmax %d", pruned, res.Stats.PrunedVisits, res.Choice, best)
	}
	if !reflect.DeepEqual(res.Target, res.policyTarget()) || !reflect.DeepEqual(plain.Visits, plain.policyTarget()) {
		t.Fatal("policyTarget is not Target under ForcedK and Visits without")
	}
	t.Logf("visits %v -> target %v (%d forced overrides)", res.Visits, res.Target, res.Stats.ForcedPlayouts)
}

// forcedConfig is pcrGame's seat with forced playouts at k (0 off).
func forcedConfig(k float64) SeatConfig {
	sc := DefaultSeatConfig()
	sc.Search.Sims, sc.Search.CPUCT = 12, 0.25
	sc.Source = testSeatSource
	sc.Explore = true
	sc.Search.ForcedK = k
	return sc
}

// An az seat with ForcedK forces playouts in its full searches only: never
// in a playout-cap cheap or a decided-game search. Its records carry the
// pruned target (no child at one visit, never more than the search's
// simulations) and the game replays exactly.
func TestSeatForcedOnlyInFullSearches(t *testing.T) {
	sc := forcedConfig(2)
	sc.FullProb, sc.CheapSims = 0.5, 6
	sc.DecidedHigh, sc.DecidedN, sc.DecidedSims, sc.DecidedWeight = 0.9, 2, 6, 0.5
	head, recs, diags, _ := pcrGame(t, sc)
	var full, other, prunedRecs int
	for _, d := range diags {
		f := !d.Cheap && !d.Decided
		if (d.Stats.ForcedSearched == 1) != f {
			t.Fatalf("a %s search (cheap %v, decided %v): forced searched %d", d.Kind, d.Cheap, d.Decided, d.Stats.ForcedSearched)
		}
		if f {
			full++
		} else {
			other++
			if d.Stats.ForcedPlayouts+d.Stats.PrunedVisits != 0 {
				t.Fatalf("a cheap or decided search forced %d and pruned %d", d.Stats.ForcedPlayouts, d.Stats.PrunedVisits)
			}
		}
	}
	if full == 0 || other == 0 {
		t.Fatalf("%d full and %d other searches: want both", full, other)
	}
	for _, r := range recs {
		sum := 0
		for _, v := range r.Visits {
			if v == 1 && r.Weight == 0 {
				t.Fatalf("seq %d: a full search's target %v keeps a one-visit child", r.Sequence, r.Visits)
			}
			sum += v
		}
		if sum > r.Sims {
			t.Fatalf("seq %d: target %v sums past %d simulations", r.Sequence, r.Visits, r.Sims)
		}
		if r.Weight == 0 && sum < r.Sims {
			prunedRecs++
		}
	}
	if prunedRecs == 0 {
		t.Fatal("no full search's record was pruned")
	}
	again, recs2, _, _ := pcrGame(t, sc)
	if again != head || len(recs2) != len(recs) {
		t.Fatalf("replay diverged: %s (%d records) vs %s (%d)", again, len(recs2), head, len(recs))
	}
	t.Logf("%d full (forced) and %d cheap or decided searches; %d of %d records pruned", full, other, prunedRecs, len(recs))
}

// With ForcedK 0 the seat does no forced work (the plain seat: the e2e check
// against the pre-option binary is in the dzgorge smoke), and ForcedK 2
// changes what is played or recorded.
func TestSeatForcedOffDoesNoForcedWork(t *testing.T) {
	want, wantRecs, diags, _ := pcrGame(t, forcedConfig(0))
	for _, d := range diags {
		if d.Stats.ForcedSearched+d.Stats.ForcedPlayouts+d.Stats.PrunedVisits+d.Stats.PrunedChildren != 0 {
			t.Fatalf("ForcedK 0 counted forced work: %+v", d.Stats)
		}
	}
	forced, frecs, _, _ := pcrGame(t, forcedConfig(2))
	if forced == want && reflect.DeepEqual(frecs, wantRecs) {
		t.Fatal("ForcedK 2 played and recorded exactly the plain game: the option did nothing")
	}
}

func TestValidateForcedK(t *testing.T) {
	for _, k := range []float64{-1, math.NaN(), math.Inf(1)} {
		o := DefaultOptions()
		o.ForcedK = k
		if err := o.Validate(nil); err == nil || !strings.Contains(err.Error(), "forced-playout") {
			t.Fatalf("ForcedK %g: %v", k, err)
		}
	}
	for _, k := range []float64{0, 0.5, 2} {
		o := DefaultOptions()
		o.ForcedK = k
		if err := o.Validate(nil); err != nil {
			t.Fatalf("ForcedK %g: %v", k, err)
		}
	}
}

// Forced playouts keep the search a pure function of its inputs, and the
// node cache stays exact under them: the forced rule reads only the root's
// tree statistics, which a cached walk rebuilds identically.
func TestForcedIsDeterministicAndCacheExact(t *testing.T) {
	forced := 0
	for i, r := range measureRoots(t, 4) {
		run := func(cache int) Result {
			opts := DefaultOptions()
			opts.Sims, opts.Seed, opts.NodeCache, opts.ForcedK, opts.CPUCT = 60, uint64(i)+11, cache, 2, 0.25
			return searchAt(t, r.e, r.d, r.bot, nil, opts)
		}
		off := run(0)
		if again := run(0); !reflect.DeepEqual(off, again) {
			t.Fatalf("%s: two forced searches differ", r.name)
		}
		forced += off.Stats.ForcedPlayouts
		for _, cache := range []int{1, 3, DefaultNodeCache} {
			if d := sameResult(off, run(cache)); d != "" {
				t.Fatalf("%s cache %d: the cached forced search differs: %s", r.name, cache, d)
			}
		}
	}
	if forced == 0 {
		t.Fatal("no root search made a forced playout")
	}
}
