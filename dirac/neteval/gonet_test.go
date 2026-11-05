package neteval

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// netPath is the network the tests load: NETEVAL_NET when set (any .dzgw
// export, to run the same checks on a trained network), else gonettest's
// deterministic synthetic network of the production widths (gonettest.Wide), written under t's temp directory.
// netsimd takes its shape (its 64-lane kernels), so LoadWeights runs it on
// the parts path and LoadGonet on gonet: both backends are exercised.
func netPath(t testing.TB) string {
	t.Helper()
	if p := os.Getenv("NETEVAL_NET"); p != "" {
		return p
	}
	p, err := gonettest.WriteShape(t.TempDir(), "net.dzgw", gonettest.Wide, policynet.EntityRawWidth, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// gonetWeights loads path through the gonet backend (LoadGonet).
func gonetWeights(t testing.TB, path string) *Weights {
	t.Helper()
	w, err := LoadGonet(path)
	if err != nil {
		t.Fatal(err)
	}
	if w.Backend() != "gonet" {
		t.Fatalf("%s loaded through %s, want gonet", path, w.Backend())
	}
	return w
}

func vanillaPool() *feat.Pool {
	specs := make([]feat.CardSpec, len(v.Table.Cards))
	for i := range specs {
		specs[i] = feat.CardSpec{Name: v.Table.Cards[i].Name}
	}
	return feat.NewPool(v.Table, specs)
}

// The gonet backend: the network loads, evaluates the materialized encoding with
// the same contract as netsimd (the value through the sigmoid, one logit
// per option), agrees with gonet's own cached Score and Value on the same inputs,
// allocates nothing per evaluation, and its content-keyed cache is exact.
func TestGonetFallback(t *testing.T) {
	path := netPath(t)
	w := gonetWeights(t, path)
	n := New(w, vanillaPool())
	if !n.Materialize {
		t.Fatal("a gonet Net is not materialized")
	}
	ref, err := gonet.Load(path) // gonet's defaults: its own caches on
	if err != nil {
		t.Fatal(err)
	}
	ps := positions(30)
	var d dirac.Decision
	var logits [dirac.MaxOptions]float32
	for i := range ps {
		s := ps[i]
		s.StepInto(&d)
		n.NewWorld()
		n.Priors(s, &d, nil, logits[:d.N])
		// the same inputs through gonet's own Score and Value
		n.encodePriors(s, &d, nil)
		st := n.pst
		want := ref.Score(st, n.popts[:d.N])
		if !reflect.DeepEqual(want, logits[:d.N]) {
			t.Fatalf("position %d: logits %v, gonet Score %v", i, logits[:d.N], want)
		}
		for _, seat := range []uint8{0, 1} {
			got := n.Value(s, &d, seat)
			n.state(s, &d, seat)
			if wv := ref.Value(n.pst); wv != got {
				t.Fatalf("position %d seat %d: value %v, gonet Value %v", i, seat, got, wv)
			}
		}
	}
	c := n.EnableCheck()
	run := func() {
		for i := range ps {
			ps[i].StepInto(&d)
			n.NewWorld()
			n.Priors(ps[i], &d, nil, logits[:d.N])
			n.Value(ps[i], &d, 1-d.Player)
		}
	}
	run()
	if c.N == 0 || c.Bad != 0 || c.MaxV != 0 || c.MaxL != 0 {
		t.Fatalf("cached vs uncached: %d checked, %d bad, max dv %g dl %g", c.N, c.Bad, c.MaxV, c.MaxL)
	}
	n.check = nil
	if a := testing.AllocsPerRun(3, run); a != 0 {
		t.Fatalf("gonet backend allocates %v per pass", a)
	}
}

// Two vanilla games, both seats searching through the network on the gonet
// backend (40 simulations, closed-list belief), play to their end, and a
// searched game replays identically.
func TestGonetPlaysGames(t *testing.T) {
	w := gonetWeights(t, netPath(t))
	for g := uint64(1); g <= 2; g++ {
		acts := searchGame(New(w, vanillaPool()), g, 1000)
		var s dirac.State
		s.Init(v.Table, v.Decks[g%5], v.Decks[(g+2)%5], g, dirac.Config{})
		var d dirac.Decision
		for _, x := range acts {
			s.StepInto(&d)
			s.Apply(x)
		}
		s.StepInto(&d)
		if d.Kind != dirac.DkNone {
			t.Fatalf("game %d: not over after %d actions (turn %d)", g, len(acts), s.Turn())
		}
		t.Logf("game %d: %d actions, %d turns", g, len(acts), s.Turn())
	}
	if a, b := searchGame(New(w, vanillaPool()), 3, 120), searchGame(New(w, vanillaPool()), 3, 120); !reflect.DeepEqual(a, b) {
		t.Fatalf("a searched game differs on replay: %d actions, then %d", len(a), len(b))
	}
}

// A file neither backend loads names both refusals.
func TestLoadWeightsRefusesBoth(t *testing.T) {
	p := filepath.Join(t.TempDir(), "x.dzgw")
	os.WriteFile(p, []byte("not a network"), 0o644)
	if _, err := LoadWeights(p); err == nil {
		t.Fatal("garbage loaded")
	} else if s := err.Error(); !strings.Contains(s, "netsimd") || !strings.Contains(s, "gonet") {
		t.Fatalf("error %q does not name both backends", s)
	}
}
