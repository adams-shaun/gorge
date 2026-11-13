//go:build dirac_mp

package neteval

import (
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
