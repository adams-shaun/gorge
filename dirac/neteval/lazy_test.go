package neteval

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/search"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// searchResults is searchGame's loop with Options.LazyPriors set from lazy,
// returning every decision's Result and the summed PriorSkips.
func searchResults(n *Net, g uint64, maxDec int, lazy bool) ([]search.Result, int) {
	opts := search.DefaultOptions()
	opts.Sims = 40
	opts.LazyPriors = lazy
	sr := search.NewSearcher(opts, n, search.Heuristic{})
	sr.Belief = search.NewBelief(v.Table, v.Decks[:])
	var s dirac.State
	s.Init(v.Table, v.Decks[g%5], v.Decks[(g+2)%5], g, dirac.Config{})
	var d dirac.Decision
	var out []search.Result
	skips := 0
	for i := 0; i < maxDec; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		r := sr.Search(&s, g*977+uint64(d.Player))
		out = append(out, *r)
		skips += r.Stats.PriorSkips
		k0, p0 := d.Kind, d.Player
		var scratch dirac.Decision
		for j := 0; j < r.NActs; j++ {
			if j > 0 && !search.Offered(&s, r.Acts[j], k0, p0, &scratch) {
				continue
			}
			s.Apply(r.Acts[j])
		}
	}
	return out, skips
}

// sameResult is sameSearch for the neteval package: every field the lazy
// prior could touch, Stats excluded.
func sameResult(a, b search.Result) bool {
	return a.N == b.N && a.Keys == b.Keys && a.Visits == b.Visits && a.Q == b.Q &&
		a.Avail == b.Avail && a.Prior == b.Prior && a.RootValue == b.RootValue &&
		a.Choice == b.Choice && a.NActs == b.NActs && a.Acts == b.Acts
}

// TestLazyPriorsSameWithNet: on the real network the lazy prior is exact at
// every decision of four games and runs fewer forward passes.
func TestLazyPriorsSameWithNet(t *testing.T) {
	for g := uint64(1); g <= 4; g++ {
		eagerNet, lazyNet := testNet(t), testNet(t)
		eager, _ := searchResults(eagerNet, g, 120, false)
		lazy, skips := searchResults(lazyNet, g, 120, true)
		if len(eager) != len(lazy) {
			t.Fatalf("game %d: %d decisions then %d", g, len(eager), len(lazy))
		}
		for i := range eager {
			if !sameResult(eager[i], lazy[i]) {
				t.Fatalf("game %d decision %d: LazyPriors changed the search", g, i)
			}
		}
		if skips == 0 {
			t.Fatalf("game %d: no point skipped the ranking", g)
		}
		if lazyNet.Evals >= eagerNet.Evals {
			t.Fatalf("game %d: forward passes lazy %d, eager %d", g, lazyNet.Evals, eagerNet.Evals)
		}
		t.Logf("game %d: %d decisions, %d skips, eager %d evals, lazy %d", g, len(eager), skips, eagerNet.Evals, lazyNet.Evals)
	}
}
