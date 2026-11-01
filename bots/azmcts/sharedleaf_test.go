package azmcts

import (
	"context"
	"math"
	"math/rand/v2"
	"os"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/rules"
)

// testNets are the networks the shared-leaf tests run on: the gonettest
// network (or AZMCTS_TEST_NET), and a pure policynet model with a value
// head.
func testNets(t *testing.T) map[string]*policynet.Model {
	t.Helper()
	path := os.Getenv("AZMCTS_TEST_NET")
	if path == "" {
		var err error
		if path, err = gonettest.Write(t.TempDir(), "leaf.dzgw"); err != nil {
			t.Fatal(err)
		}
	}
	gn, err := gonet.Model(path)
	if err != nil {
		t.Fatal(err)
	}
	m := policynet.NewModel(policynet.TableRows, 4, 3, rand.New(rand.NewPCG(31, 32)))
	m.InitValue(3, rand.New(rand.NewPCG(33, 34)))
	return map[string]*policynet.Model{"gonet": gn, "policynet": m}
}

// TestSharedLeafMatchesLeafValue pins priorsWithLeaf: at every decision of
// whole bot games that enumerates for the deciding seat, the value the
// point's prior carries out (rankedPriorLeaf) is bit-identical to
// leafValue's own projection, encoding and value head for that seat, and
// the prior itself is the one rankedPrior computes.
func TestSharedLeafMatchesLeafValue(t *testing.T) {
	for name, net := range testNets(t) {
		sc := new(viewScratch)
		var scratch boardScratch
		checked := 0
		for _, s := range []uint64{testSeed, testSeed + 1, testSeed + 2} {
			cfg := testConfig(t, "mono-red-prowess", "mono-blue-tempo", s)
			e := rules.New(cfg)
			e.Advance()
			rngs := searchprobe.BotRandoms(cfg.Seed, 2)
			board := botpolicy.NewBoard(2)
			for steps := 0; steps < 2500 && !e.G.Over; steps++ {
				d := e.Pending()
				if d == nil {
					t.Fatalf("no pending decision at step %d", steps)
				}
				in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
				obs := searchprobe.NewCollector(d.Player)
				if cands, kind, _, ok, _ := enumerateCutInto(obs, e, d, in, FullKinds(), BenchCandidateLimit, true, &scratch); ok {
					want := leafValue(net, e, d.Player, sc)
					_, p0, fell0, b0 := rankedPrior(net, 6, 0, e, d, in, kind, cands, &sc.prior)
					var got float64
					_, p1, fell1, b1 := rankedPriorLeaf(net, 6, 0, e, d, in, kind, cands, &sc.prior, &got)
					if math.Float64bits(got) != math.Float64bits(want) {
						t.Fatalf("%s seed %d step %d seat %d: shared leaf %v, leafValue %v", name, s, steps, d.Player, got, want)
					}
					if !reflect.DeepEqual(p0, p1) || fell0 != fell1 || b0 != b1 {
						t.Fatalf("%s seed %d step %d: the leaf read changed the prior: %v vs %v", name, s, steps, p0, p1)
					}
					checked++
				}
				if err := e.Submit(in); err != nil {
					t.Fatalf("step %d submit: %v", steps, err)
				}
			}
		}
		if checked < 60 {
			t.Fatalf("%s: only %d points compared", name, checked)
		}
		t.Logf("%s: %d points compared", name, checked)
	}
}

// TestSharedLeafMatchesSeparate pins engineEnv.leafV at the search level: a
// network search (re-dealt worlds, opponent points,
// auto-pay, top-k), with and without the node cache over cached worlds,
// returns the same Result -- visits, values, choice and every counter --
// whether a point's leaf is read off its prior or projected and encoded
// again.
func TestSharedLeafMatchesSeparate(t *testing.T) {
	defer searchprobe.SetRedealProbeVerify(searchprobe.SetRedealProbeVerify(false))
	for name, net := range testNets(t) {
		opts := DefaultOptions()
		opts.Sims, opts.Seed = 60, 7
		opts.AutoPayment, opts.OpponentNodes, opts.OpponentMirror, opts.PriorTopK = true, true, true, 6
		opts.Kinds = FullKinds()
		for _, s := range []uint64{testSeed, testSeed + 1} {
			p := feedPosition(t, testConfig(t, "mono-red-prowess", "mono-blue-tempo", s), 5, 1, nil)
			// rankMax 3 skips the ranking at most points (rankSkips): those
			// never read the leaf, so they must not claim it.
			for _, c := range []struct{ cw, rankMax int }{{0, 0}, {1, 0}, {0, 3}, {1, 3}} {
				cw := c.cw
				run := func(shared bool) Result {
					defer func(old bool) { noSharedLeaf = old }(noSharedLeaf)
					noSharedLeaf = !shared
					o := opts
					o.CachedWorlds, o.PriorRankMaxOptions = cw, c.rankMax
					obs := searchprobe.NewCollector(0)
					src, err := NewRedeal(p.input(t, p.e), obs, s, 0)
					if err != nil {
						t.Fatal(err)
					}
					res, err := Search(context.Background(), Root{Engine: p.e, Decision: p.d, Bot: p.bot, Observer: obs}, src, net, o)
					if err != nil {
						t.Fatal(err)
					}
					return res
				}
				separate, shared := run(false), run(true)
				if separate.Stats.Completed == 0 || separate.Stats.Expanded == 0 {
					t.Fatalf("%s seed %d cw %d: no simulation completed", name, s, cw)
				}
				if c.rankMax > 0 && separate.Stats.PriorFallbacks == 0 {
					t.Fatalf("%s seed %d cw %d: rankMax %d skipped no ranking", name, s, cw, c.rankMax)
				}
				if !reflect.DeepEqual(separate, shared) {
					t.Fatalf("%s seed %d cw %d rankMax %d: the shared leaf changed the search\nseparate %+v\nshared   %+v", name, s, cw, c.rankMax, separate, shared)
				}
			}
		}
	}
}
