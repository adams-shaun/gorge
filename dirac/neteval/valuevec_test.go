package neteval

import (
	"math"
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots/gonet/gonettest"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// valueKNet is a Net over a network with a K-way value head of k outcomes:
// NETEVAL_VALUEK_NET's export when set (a trained dzg value_k network),
// else gonettest's synthetic one at the production widths.
func valueKNet(t testing.TB, k int) *Net {
	t.Helper()
	path := os.Getenv("NETEVAL_VALUEK_NET")
	if path == "" {
		lay := gonettest.DefaultLayout
		lay.ValueK = k
		var err error
		if path, err = gonettest.WriteLayout(t.TempDir(), "k.dzgw", gonettest.Wide, policynet.EntityRawWidth, nil, lay); err != nil {
			t.Fatal(err)
		}
	}
	w, err := LoadWeights(path)
	if err != nil {
		t.Fatal(err)
	}
	if w.Backend() != "gonet" || w.ValueK() == 0 {
		t.Fatalf("%s: backend %s, value_k %d; a K-way network loads on gonet", path, w.Backend(), w.ValueK())
	}
	specs := make([]feat.CardSpec, len(v.Table.Cards))
	for i := range specs {
		specs[i] = feat.CardSpec{Name: v.Table.Cards[i].Name}
	}
	return New(w, feat.NewPool(v.Table, specs))
}

// checkVec holds a written value vector to search.VecEvaluator's contract
// at s: each entry in [0,1], a lost seat's 0, nothing past the game's seats,
// and the entries summing to 1 while every seat lives (the head's mass on a
// lost seat is dropped, not spread: a trained head puts none there).
func checkVec(t testing.TB, s *dirac.State, seat uint8, out *[dirac.MaxSeats]float64) {
	t.Helper()
	n := s.NumSeats()
	sum := 0.0
	for r, x := range out {
		if x < 0 || x > 1 || (r >= int(n) && x != 0) {
			t.Fatalf("vector %v at %d seats: entry %d out of contract", *out, n, r)
		}
		sum += x
	}
	lost := false
	for p := uint8(0); p < n; p++ {
		if s.FeatLost(p) {
			lost = true
			if out[(p+n-seat)%n] != 0 {
				t.Fatalf("vector %v: lost seat %d has mass", *out, p)
			}
		}
	}
	if sum > 1+1e-5 || (!lost && math.Abs(sum-1) > 1e-5) {
		t.Fatalf("vector %v sums to %v", *out, sum)
	}
}

// TestValueVecReadsTheHead: a K-way network's Net reports a vector for the
// build's seat count (HasValueVec), writes one that keeps the contract for
// every seat's view of positions along a game, answers a repeat from its
// cache bit for bit, and allocates nothing once warm. A scalar network's Net
// reports none.
func TestValueVecReadsTheHead(t *testing.T) {
	n := valueKNet(t, int(dirac.MaxSeats)+1)
	if !n.HasValueVec(dirac.MaxSeats) {
		t.Fatalf("value_k %d: no vector for %d seats", n.w.ValueK(), dirac.MaxSeats)
	}
	if testNet(t).HasValueVec(2) {
		t.Fatal("a scalar network reports a value vector")
	}
	seats := uint8(dirac.MaxSeats)
	var decks [dirac.MaxSeats][]dirac.CardID
	for i := uint8(0); i < seats; i++ {
		decks[i] = v.Decks[int(i)%len(v.Decks)]
	}
	var s dirac.State
	s.InitN(v.Table, decks, seats, 7, dirac.Config{})
	pol := dirac.NewRandomPolicy(7)
	var d dirac.Decision
	var out, again [dirac.MaxSeats]float64
	checked := 0
	for i := 0; i < 300; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if i%5 == 0 {
			for seat := uint8(0); seat < seats; seat++ {
				n.ValueVec(&s, &d, seat, &out)
				checkVec(t, &s, seat, &out)
				evals := n.Evals
				n.ValueVec(&s, &d, seat, &again)
				if again != out || n.Evals != evals {
					t.Fatalf("a repeat evaluated again or answered %v, not %v", again, out)
				}
				checked++
			}
		}
		s.Apply(pol.Pick(&d))
	}
	if checked == 0 {
		t.Fatal("no position evaluated")
	}
	s.StepInto(&d)
	if d.Kind != dirac.DkNone {
		n.cache = nil // every call encodes and runs the head
		if a := testing.AllocsPerRun(20, func() { n.ValueVec(&s, &d, d.Player, &out) }); a != 0 {
			t.Errorf("ValueVec allocates %v per call", a)
		}
	}
}
