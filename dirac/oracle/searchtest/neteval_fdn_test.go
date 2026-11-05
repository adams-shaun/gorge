package searchtest

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/neteval"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/search"
)

// TestSearchEvaluatorExactFDN searches FDN games with the network
// evaluator and re-evaluates every position it is asked through a
// reference evaluator (the materialized encoding, the view built in full,
// no cache): the content cache keys, the base and line-of-play views and
// the parts must give the reference's values and logits (to float
// rounding), and no content key may name two encodings.
func TestSearchEvaluatorExactFDN(t *testing.T) {
	t.Parallel()

	net := netPath(t)
	c, pool := testCorpus(t)
	w, err := neteval.LoadWeights(net)
	if err != nil {
		t.Fatal(err)
	}
	opts := search.DefaultOptions()
	opts.Sims = 24
	games, decisions := 0, 0
	var checks []*neteval.Check
	var nets []*neteval.Net
	for _, p := range oracle.Pairings(pool, 12, 41) {
		m, tbl, err := oracle.SetMatchup(rawSet("fdn"), c, p.A, p.B, 0)
		if err != nil {
			continue
		}
		fp, err := oracle.FeatPool(c.Reg, tbl, nil)
		if err != nil {
			t.Fatal(err)
		}
		side, err := oracle.RealKernel(tbl, nil, oracle.Config{PassElision: true})(m, p.Seed)
		if err != nil {
			continue
		}
		ks, kd, _ := oracle.KernelStateOf(side)
		var srs [2]*search.Searcher
		for i := range srs {
			n := neteval.New(w, fp)
			n.VerifyKeys = true
			checks = append(checks, n.EnableCheck())
			nets = append(nets, n)
			srs[i] = search.NewSearcher(opts, n, search.Heuristic{})
		}
		games++
		func() {
			defer func() {
				if x := recover(); x != nil {
					t.Logf("%s vs %s: stopped: %v", p.A, p.B, x)
				}
			}()
			for i := 0; i < 120; i++ {
				ks.StepInto(kd)
				if kd.Kind == dirac.DkNone {
					return
				}
				res := srs[kd.Player].Search(ks, p.Seed*31+uint64(i))
				decisions++
				k0, p0 := kd.Kind, kd.Player
				var scratch dirac.Decision
				for j := 0; j < res.NActs; j++ {
					if j > 0 && !search.Offered(ks, res.Acts[j], k0, p0, &scratch) {
						continue
					}
					ks.Apply(res.Acts[j])
				}
			}
		}()
	}
	var n, bad, conf, keys int
	var maxV, maxL float64
	for i, c := range checks {
		n += c.N
		bad += c.Bad
		maxV = max(maxV, c.MaxV)
		maxL = max(maxL, c.MaxL)
		conf += nets[i].KeyConflicts
		keys += nets[i].KeyChecks
	}
	t.Logf("%d FDN games, %d searched decisions: %d evaluations checked (max |value diff| %.3g, max |logit diff| %.3g, %d off), %d content keys, %d naming two encodings", games, decisions, n, maxV, maxL, bad, keys, conf)
	if games == 0 {
		t.Skip("no FDN matchup built")
	}
	if bad > 0 || conf > 0 {
		t.Fatalf("%d evaluations differ from the reference, %d content keys name two encodings", bad, conf)
	}
}
