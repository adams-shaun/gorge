package azmcts

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/searchprobe"
)

// TestLazyPriorMatchesEager pins walkConfig.lazyPrior: a network search over
// re-dealt worlds, with opponent points, auto-pay and a top-k prior, returns the same Result -- visits, values, choice and every
// counter -- whether in-walk priors are deferred or computed at every point.
// AZMCTS_TEST_NET=<a .dzgw> runs it on a real network instead of the
// gonettest one.
func TestLazyPriorMatchesEager(t *testing.T) {
	defer searchprobe.SetRedealProbeVerify(searchprobe.SetRedealProbeVerify(false))
	path := os.Getenv("AZMCTS_TEST_NET")
	if path == "" {
		var err error
		if path, err = gonettest.Write(t.TempDir(), "lazy.dzgw"); err != nil {
			t.Fatal(err)
		}
	}
	net, err := gonet.Model(path)
	if err != nil {
		t.Fatal(err)
	}
	opts := DefaultOptions()
	opts.Sims, opts.Seed = 100, 7
	opts.AutoPayment, opts.OpponentNodes, opts.OpponentMirror, opts.PriorTopK = true, true, true, 6
	opts.Kinds = FullKinds()
	for _, c := range []struct {
		s       uint64
		rankMax int
	}{{testSeed, 0}, {testSeed + 1, 0}, {testSeed, 3}} {
		s := c.s
		opts := opts
		// rankMax 3 skips the ranking at most points (rankSkips), in the
		// deferred closure as in the eager prior.
		opts.PriorRankMaxOptions = c.rankMax
		p := feedPosition(t, testConfig(t, "mono-red-prowess", "mono-blue-tempo", s), 5, 1, nil)
		run := func(eager bool) Result {
			defer func(old bool) { eagerPrior = old }(eagerPrior)
			eagerPrior = eager
			obs := searchprobe.NewCollector(0)
			src, err := NewRedeal(p.input(t, p.e), obs, s, 0)
			if err != nil {
				t.Fatal(err)
			}
			res, err := Search(context.Background(), Root{Engine: p.e, Decision: p.d, Bot: p.bot, Observer: obs}, src, net, opts)
			if err != nil {
				t.Fatal(err)
			}
			return res
		}
		eager, lazy := run(true), run(false)
		if eager.Stats.Completed == 0 {
			t.Fatalf("seed %d: no simulation completed", s)
		}
		if c.rankMax > 0 && eager.Stats.PriorFallbacks == 0 {
			t.Fatalf("seed %d: rankMax %d skipped no ranking", s, c.rankMax)
		}
		if !reflect.DeepEqual(eager, lazy) {
			t.Fatalf("seed %d rankMax %d: lazy prior changed the search\neager %+v\nlazy  %+v", s, c.rankMax, eager, lazy)
		}
	}
}
