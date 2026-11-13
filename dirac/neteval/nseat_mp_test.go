//go:build dirac_mp

package neteval

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
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
