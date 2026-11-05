package neteval

import (
	"math"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/search"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// testNet is a Net over netPath's network, loaded as LoadWeights picks
// (netsimd for the synthetic network), and the vanilla table's feat pool.
func testNet(t testing.TB) *Net {
	w, err := LoadWeights(netPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if w.Backend() != "netsimd" && os.Getenv("NETEVAL_NET") == "" {
		t.Fatalf("the synthetic network loaded through %s, want netsimd", w.Backend())
	}
	specs := make([]feat.CardSpec, len(v.Table.Cards))
	for i := range specs {
		specs[i] = feat.CardSpec{Name: v.Table.Cards[i].Name}
	}
	return New(w, feat.NewPool(v.Table, specs))
}

func positions(n int) []*dirac.State {
	var out []*dirac.State
	for g := uint64(1); len(out) < n; g++ {
		var s dirac.State
		s.Init(v.Table, v.Decks[g%5], v.Decks[(g+1)%5], g, dirac.Config{})
		pol := dirac.NewRandomPolicy(g)
		var d dirac.Decision
		for i := 0; i < 300 && len(out) < n; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if i%5 == 2 {
				c := new(dirac.State)
				c.CopyFrom(&s)
				out = append(out, c)
			}
			s.Apply(pol.Pick(&d))
		}
	}
	return out
}

func TestZeroAllocsPerEval(t *testing.T) {
	n := testNet(t)
	ps := positions(40)
	var d dirac.Decision
	var logits [dirac.MaxOptions]float32
	run := func() {
		for i := range ps {
			ps[i].StepInto(&d)
			n.NewWorld()
			n.Priors(ps[i], &d, nil, logits[:d.N])
			n.Value(ps[i], &d, 1-d.Player)
		}
	}
	run() // grow the scratch
	if a := testing.AllocsPerRun(5, run); a != 0 {
		t.Fatalf("allocs per pass over %d positions = %v", len(ps), a)
	}
}

func BenchmarkEval(b *testing.B) {
	n := testNet(b)
	ps := positions(64)
	var d dirac.Decision
	var logits [dirac.MaxOptions]float32
	b.ReportAllocs()
	opts := 0
	for i := 0; i < b.N; i++ {
		s := ps[i%len(ps)]
		s.StepInto(&d)
		n.Priors(s, &d, nil, logits[:d.N])
		opts += int(d.N)
	}
	b.ReportMetric(float64(opts)/float64(b.N), "options/eval")
}

// searchGame plays one vanilla game with both seats searching through n
// (closed-list belief, c6 options at 40 simulations) and returns its
// actions.
func searchGame(n *Net, g uint64, maxDec int) []dirac.Action {
	opts := search.DefaultOptions()
	opts.Sims = 40
	sr := search.NewSearcher(opts, n, search.Heuristic{})
	sr.Belief = search.NewBelief(v.Table, v.Decks[:])
	var s dirac.State
	s.Init(v.Table, v.Decks[g%5], v.Decks[(g+2)%5], g, dirac.Config{})
	var d dirac.Decision
	var acts []dirac.Action
	for i := 0; i < maxDec; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		r := sr.Search(&s, g*977+uint64(d.Player))
		k0, p0 := d.Kind, d.Player
		var scratch dirac.Decision
		for j := 0; j < r.NActs; j++ {
			if j > 0 && !search.Offered(&s, r.Acts[j], k0, p0, &scratch) {
				continue
			}
			acts = append(acts, r.Acts[j])
			s.Apply(r.Acts[j])
		}
	}
	return acts
}

// The content key names exactly one encoding (no content key seen with two
// encodings) over searched games, and a search plays the same game under
// either key.
func TestContentKeysAreExact(t *testing.T) {
	n := testNet(t)
	n.VerifyKeys = true
	for g := uint64(1); g <= 4; g++ {
		searchGame(n, g, 120)
	}
	t.Logf("%d content keys checked, %d evaluations, %d hits", n.KeyChecks, n.Evals, n.Hits)
	if n.KeyChecks == 0 || n.KeyConflicts != 0 {
		t.Fatalf("%d of %d content keys named two encodings", n.KeyConflicts, n.KeyChecks)
	}
	for g := uint64(5); g <= 6; g++ {
		a := testNet(t)
		b := testNet(t)
		b.Keys = KeyEncoded
		ga, gb := searchGame(a, g, 80), searchGame(b, g, 80)
		if len(ga) != len(gb) {
			t.Fatalf("game %d: %d vs %d actions", g, len(ga), len(gb))
		}
		for i := range ga {
			if ga[i] != gb[i] {
				t.Fatalf("game %d: action %d differs", g, i)
			}
		}
		t.Logf("game %d: %d actions alike; evals content %d / encoded %d", g, len(ga), a.Evals, b.Evals)
	}
}

// checker is an Evaluator that, before every evaluation of n, encodes the
// position twice -- against the search base and in full -- and counts
// differences in the state or option encodings.
type checker struct {
	*Net
	full      *feat.Encoder
	fopts     []feat.Option
	gopts     []feat.Option
	n, diff   int
	firstDiff string
}

func (c *checker) check(s *dirac.State, d *dirac.Decision, seat uint8, opts bool) {
	a := c.enc.EncodeStateFor(s, d, seat)
	b := c.full.EncodeStateFor(s, d, seat)
	c.n++
	bad := ""
	switch {
	case a.Dense != b.Dense:
		bad = "dense"
	case len(a.Sparse) != len(b.Sparse):
		bad = "sparse length"
	case len(a.Cards) != len(b.Cards):
		bad = "card count"
	}
	if bad == "" {
		for i := range a.Sparse {
			if a.Sparse[i] != b.Sparse[i] {
				bad = "sparse"
			}
		}
		for i := range a.Cards {
			if a.Cards[i] != b.Cards[i] {
				bad = "card"
			}
		}
	}
	if bad == "" && opts && d.Player == seat {
		fa := c.enc.EncodeOptionsBot(d, c.fopts[:0], -1)
		fb := c.full.EncodeOptionsBot(d, c.gopts[:0], -1)
		for i := range fa {
			if fa[i] != fb[i] {
				bad = "option"
			}
		}
	}
	if bad != "" {
		c.diff++
		if c.firstDiff == "" {
			c.firstDiff = bad
		}
	}
}

func (c *checker) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	c.check(s, d, seat, false)
	return c.Net.Value(s, d, seat)
}

func (c *checker) Priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32) {
	c.check(s, d, d.Player, true)
	c.Net.Priors(s, d, bot, out)
}

// The view built against the search base (feat Encoder.SetBase, from the
// dirac's change marks) encodes every searched position exactly as the
// full build does.
func TestBaseViewMatchesFull(t *testing.T) {
	n := testNet(t)
	c := &checker{Net: n, full: feat.NewEncoder(n.enc.Pool()),
		fopts: make([]feat.Option, dirac.MaxOptions), gopts: make([]feat.Option, dirac.MaxOptions)}
	opts := search.DefaultOptions()
	opts.Sims = 30
	for g := uint64(1); g <= 6; g++ {
		sr := search.NewSearcher(opts, c, search.Heuristic{})
		sr.Belief = search.NewBelief(v.Table, v.Decks[:])
		var s dirac.State
		s.Init(v.Table, v.Decks[g%5], v.Decks[(g+3)%5], g, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 150; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			r := sr.Search(&s, g*31+uint64(d.Player))
			k0, p0 := d.Kind, d.Player
			var scratch dirac.Decision
			for j := 0; j < r.NActs; j++ {
				if j > 0 && !search.Offered(&s, r.Acts[j], k0, p0, &scratch) {
					continue
				}
				s.Apply(r.Acts[j])
			}
		}
	}
	st := n.enc.IncStats
	t.Logf("%d encodings compared, %d built against the base; card views: %d from the content cache, %d built", c.n, st.BaseBuilds, st.CardHits, st.CardBuilds)
	if c.diff > 0 {
		t.Fatalf("%d of %d encodings differ (first: %s)", c.diff, c.n, c.firstDiff)
	}
}

// pair evaluates through a (parts) and b (materialized) and records the
// largest difference.
type pair struct {
	a, b       *Net
	maxV, maxL float64
	n          int
}

func (p *pair) HasPrior() bool { return true }
func (p *pair) NewWorld()      { p.a.NewWorld(); p.b.NewWorld() }
func (p *pair) SetBase(s *dirac.State) {
	p.a.SetBase(s)
	p.b.SetBase(s)
}

func (p *pair) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	va, vb := p.a.Value(s, d, seat), p.b.Value(s, d, seat)
	p.maxV = math.Max(p.maxV, math.Abs(float64(va-vb)))
	p.n++
	return vb
}

func (p *pair) Priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32) {
	m := int(d.N)
	tmp := make([]float32, m)
	p.a.Priors(s, d, bot, tmp)
	p.b.Priors(s, d, bot, out[:m])
	for i := range tmp {
		p.maxL = math.Max(p.maxL, math.Abs(float64(tmp[i]-out[i])))
	}
	p.n++
}

// The parts path (per-object row sums and card vectors, summed in another
// order) gives the materialized encoding's values and logits to float
// rounding over searched games.
func TestPartsMatchMaterialized(t *testing.T) {
	a, b := testNet(t), testNet(t)
	b.Materialize = true
	p := &pair{a: a, b: b}
	opts := search.DefaultOptions()
	opts.Sims = 30
	for g := uint64(1); g <= 5; g++ {
		sr := search.NewSearcher(opts, p, search.Heuristic{})
		sr.Belief = search.NewBelief(v.Table, v.Decks[:])
		var s dirac.State
		s.Init(v.Table, v.Decks[g%5], v.Decks[(g+4)%5], g, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 150; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			r := sr.Search(&s, g*7+uint64(d.Player))
			k0, p0 := d.Kind, d.Player
			var scratch dirac.Decision
			for j := 0; j < r.NActs; j++ {
				if j > 0 && !search.Offered(&s, r.Acts[j], k0, p0, &scratch) {
					continue
				}
				s.Apply(r.Acts[j])
			}
		}
	}
	st := a.enc.PartsStats
	t.Logf("%d evaluations: max |value diff| %.3g, max |logit diff| %.3g; parts: %d encodes, %d objects rebuilt, %d reused", p.n, p.maxV, p.maxL, st.Encodes, st.Rebuilt, st.Reused)
	if p.maxV > 1e-5 || p.maxL > 1e-4 {
		t.Fatalf("parts differ from the materialized encoding: value %v logit %v", p.maxV, p.maxL)
	}
}
