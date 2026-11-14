//go:build dirac_mp

package neteval

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/search"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestNSeatPartsMatchMaterialized: at three and four seats every seat's
// value and priors through the parts path (its per-seat caches) equal the
// materialized encoding's, evaluated for the deciding seat and for every
// other seat at each decision, so no seat's cached rows stand in for
// another's.
func TestNSeatPartsMatchMaterialized(t *testing.T) {
	a, b := testNet(t), testNet(t)
	b.Materialize = true
	p := &pair{a: a, b: b}
	var seen [dirac.MaxSeats]bool
	for g := uint64(1); g <= 4; g++ {
		n := uint8(3 + g%2)
		var decks [dirac.MaxSeats][]dirac.CardID
		for i := uint8(0); i < n; i++ {
			decks[i] = v.Decks[(int(g)+int(i))%len(v.Decks)]
		}
		var s dirac.State
		s.InitN(v.Table, decks, n, g, dirac.Config{})
		pol := dirac.NewRandomPolicy(g)
		var d dirac.Decision
		for i := 0; i < 400; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if i%3 == 0 {
				p.NewWorld()
				for seat := uint8(0); seat < n; seat++ {
					p.Value(&s, &d, seat)
					seen[seat] = true
				}
				out := make([]float32, d.N)
				p.Priors(&s, &d, nil, out)
				seen[d.Player] = true
			}
			s.Apply(pol.Pick(&d))
		}
	}
	for seat, ok := range seen {
		if !ok {
			t.Fatalf("seat %d never evaluated", seat)
		}
	}
	t.Logf("%d evaluations: max |value diff| %.3g, max |logit diff| %.3g", p.n, p.maxV, p.maxL)
	if p.maxV > 1e-5 || p.maxL > 1e-4 {
		t.Fatalf("parts differ from the materialized encoding: value %v logit %v", p.maxV, p.maxL)
	}
}

// TestNSeatSearchWithNet: the net is every seat's evaluator in three- and
// four-seat pods searched to the end, paranoid and max^n, on one line of
// play per search (the path whose view-hash cache held two seats' parts and
// panicked at seat 2), and two runs of a pod are the same game.
func TestNSeatSearchWithNet(t *testing.T) {
	play := func(g uint64, maxN bool) (uint64, int) {
		n := testNet(t)
		o := search.DefaultOptions()
		o.Sims, o.MaxN = 8, maxN
		sr := search.NewSearcher(o, n, search.Heuristic{})
		seats := uint8(3 + g%2)
		var decks [dirac.MaxSeats][]dirac.CardID
		for i := uint8(0); i < seats; i++ {
			decks[i] = v.Decks[(int(g)+int(i))%len(v.Decks)]
		}
		var s dirac.State
		s.InitN(v.Table, decks, seats, g, dirac.Config{})
		var d dirac.Decision
		h, searched := uint64(0), 0
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			r := sr.Search(&s, g*131+uint64(i))
			searched += r.Stats.Searched
			h = h*0x100000001b3 ^ uint64(r.Acts[0])
			r.Play(&s)
		}
		return h ^ uint64(s.Winner()+1)<<56, searched
	}
	for g := uint64(1); g <= 2; g++ {
		for _, maxN := range []bool{false, true} {
			a, sa := play(g, maxN)
			b, _ := play(g, maxN)
			if a != b {
				t.Fatalf("pod %d maxn %v: two runs differ (%x, %x)", g, maxN, a, b)
			}
			if sa == 0 {
				t.Fatalf("pod %d maxn %v: no decision searched", g, maxN)
			}
		}
	}
}

// TestFourSeatRootEvalEverySeat: at a root of a four-seat Commander game
// whose seats' libraries differ (so every seat's view differs) the net
// values the root from each seat's view without panicking, through the
// parts path and the materialized encoding alike, and the four values are
// finite and pairwise distinct: each seat reads its own seat-relative
// blocks, not another seat's. Priors at the root match too, and a pass of
// every seat's value and the decider's priors allocates nothing.
func TestFourSeatRootEvalEverySeat(t *testing.T) {
	a, b := testNet(t), testNet(t)
	b.Materialize = true
	var decks [dirac.MaxSeats][]dirac.CardID
	var cmds [dirac.MaxSeats][2]int16
	for p := 0; p < 4; p++ {
		decks[p] = append([]dirac.CardID{v.GrizzlyBears}, v.Decks[p][:40-3*p]...)
		cmds[p] = [2]int16{0, -1}
	}
	var s dirac.State
	s.InitN(v.Table, decks, 4, 7, dirac.Config{NoElide: true, Commander: true, Commanders: cmds})
	// the root: the first decision offering several options, reached by
	// a random line (the libraries still differ there)
	pol := dirac.NewRandomPolicy(7)
	var d dirac.Decision
	for s.StepInto(&d); d.Kind != dirac.DkNone && d.N < 3; s.StepInto(&d) {
		s.Apply(pol.Pick(&d))
	}
	if d.Kind == dirac.DkNone || s.NumSeats() != 4 {
		t.Fatalf("setup: decision %v at %d seats", d.Kind, s.NumSeats())
	}
	var vals [4]float32
	for seat := uint8(0); seat < 4; seat++ {
		a.NewWorld()
		b.NewWorld()
		va, vb := a.Value(&s, &d, seat), b.Value(&s, &d, seat)
		if math.IsNaN(float64(va)) || math.IsInf(float64(va), 0) {
			t.Fatalf("seat %d: value %v", seat, va)
		}
		if math.Abs(float64(va-vb)) > 1e-5 {
			t.Fatalf("seat %d: parts value %v, materialized %v", seat, va, vb)
		}
		vals[seat] = va
	}
	for p := 0; p < 4; p++ {
		for q := p + 1; q < 4; q++ {
			if vals[p] == vals[q] {
				t.Fatalf("seats %d and %d value the root the same (%v): %v", p, q, vals[p], vals)
			}
		}
	}
	pa, pb := make([]float32, d.N), make([]float32, d.N)
	a.Priors(&s, &d, nil, pa)
	b.Priors(&s, &d, nil, pb)
	for i := range pa {
		if math.Abs(float64(pa[i]-pb[i])) > 1e-4 {
			t.Fatalf("option %d: parts logit %v, materialized %v", i, pa[i], pb[i])
		}
	}
	run := func() {
		a.NewWorld()
		for seat := uint8(0); seat < 4; seat++ {
			a.Value(&s, &d, seat)
		}
		a.Priors(&s, &d, nil, pa)
	}
	run()
	if n := testing.AllocsPerRun(5, run); n != 0 {
		t.Fatalf("allocs per four-seat root pass = %v", n)
	}
	t.Logf("root values by seat %v over %d options", vals, d.N)
}
