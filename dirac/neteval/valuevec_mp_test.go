//go:build dirac_mp

package neteval

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/search"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// vecProbe is a Net whose ValueVec checks every vector the search reads.
type vecProbe struct {
	*Net
	t           testing.TB
	reads, four int
}

func (p *vecProbe) ValueVec(s *dirac.State, d *dirac.Decision, seat uint8, out *[dirac.MaxSeats]float64) {
	p.Net.ValueVec(s, d, seat, out)
	checkVec(p.t, s, seat, out)
	p.reads++
	if s.NumSeats() == 4 {
		live := 0
		for _, x := range out {
			if x > 0 {
				live++
			}
		}
		if live == 4 {
			p.four++
		}
	}
}

// TestMaxNLeafReadsTheValueHead: under Options.MaxN a four-seat pod's
// search reads the K-way head's vector at its leaves (Stats.VecLeaves, a
// four-entry vector), while a scalar network and a head too narrow for the
// pod (value_k 3: two seats and no winner) fall back to the scalar value and
// the even split, reading no vector. NETEVAL_VALUEK_NET runs the first case
// on a trained export.
func TestMaxNLeafReadsTheValueHead(t *testing.T) {
	pod := func(ev search.Evaluator) (vecLeaves, maxN int) {
		o := search.DefaultOptions()
		o.Sims, o.MaxN = 16, true
		sr := search.NewSearcher(o, ev, search.Heuristic{})
		var decks [dirac.MaxSeats][]dirac.CardID
		for i := 0; i < 4; i++ {
			decks[i] = v.Decks[(i+1)%len(v.Decks)]
		}
		var s dirac.State
		s.InitN(v.Table, decks, 4, 3, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 400; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			r := sr.Search(&s, 977+uint64(i))
			vecLeaves += r.Stats.VecLeaves
			maxN += r.Stats.MaxN
			r.Play(&s)
		}
		return vecLeaves, maxN
	}
	p := &vecProbe{Net: valueKNet(t, 5), t: t}
	leaves, maxN := pod(p)
	if maxN == 0 || leaves == 0 || p.reads != leaves || p.four == 0 {
		t.Fatalf("K-way net: %d max^n searches, %d vector leaves, %d reads, %d four-seat vectors", maxN, leaves, p.reads, p.four)
	}
	t.Logf("K-way net: %d max^n searches read %d leaf vectors (%d with four living seats)", maxN, leaves, p.four)
	if os.Getenv("NETEVAL_VALUEK_NET") != "" {
		return
	}
	for _, c := range []struct {
		name string
		ev   search.Evaluator
	}{{"scalar", testNet(t)}, {"value_k 3", valueKNet(t, 3)}} {
		if leaves, maxN := pod(c.ev); maxN == 0 || leaves != 0 {
			t.Errorf("%s net: %d max^n searches, %d vector leaves; want the scalar fallback", c.name, maxN, leaves)
		}
	}
}
