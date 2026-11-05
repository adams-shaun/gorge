package searchtest

import (
	"cmp"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/neteval"
)

// TestPartsMatchStateFDN replays recorded FDN search calls through one
// long-lived parts encoder (its caches and per-object memo warm, the base
// set as the search sets it) and asserts at every call that the parts are
// exactly the full encoding of a fresh encoder: the dense vector, the
// multiset of sparse entries (each object's rows plus the fresh ones) and
// the entity cards in order, a hand card's castable bit applied.
func TestPartsMatchStateFDN(t *testing.T) {
	t.Parallel()

	net := netPath(t)
	w, err := neteval.LoadWeights(net)
	if err != nil {
		t.Fatal(err)
	}
	sets := recordFDNTraces(t, w, 6, 3000)
	if len(sets) == 0 {
		t.Skip("no FDN matchup built")
	}
	enc := feat.NewEncoder(sets[0].pool)
	var got, want []feat.Feature
	byRow := func(a, b feat.Feature) int {
		if c := cmp.Compare(a.Row, b.Row); c != 0 {
			return c
		}
		return cmp.Compare(a.Value, b.Value)
	}
	calls := 0
	for si, ts := range sets {
		enc.SetPool(ts.pool)
		ref := feat.NewEncoder(ts.pool)
		for i := range ts.tr.evs {
			ev := &ts.tr.evs[i]
			s := ts.tr.states[ev.st]
			switch ev.kind {
			case 0:
				enc.SetBase(s)
				continue
			case 1:
				continue
			}
			seat := ev.seat
			if ev.kind == 3 {
				seat = ev.d.Player
			}
			p := enc.EncodeParts(s, &ev.d, seat)
			st := ref.EncodeStateFor(s, &ev.d, seat)
			calls++
			if p.Dense != st.Dense {
				t.Fatalf("set %d event %d: dense differs", si, i)
			}
			got = append(got[:0], p.Fresh...)
			for _, o := range p.Objs {
				got = append(got, o.Rows...)
			}
			want = append(want[:0], st.Sparse...)
			slices.SortFunc(got, byRow)
			slices.SortFunc(want, byRow)
			if !slices.Equal(got, want) {
				t.Fatalf("set %d event %d: sparse multiset differs (%d vs %d entries)", si, i, len(got), len(want))
			}
			if len(p.Ents) != len(st.Cards) {
				t.Fatalf("set %d event %d: %d entity cards, want %d", si, i, len(p.Ents), len(st.Cards))
			}
			for k := range p.Ents {
				pe := &p.Ents[k]
				c := *pe.Card
				if pe.Castable {
					c.Raw[feat.EntCastable] = 1
				}
				if c != st.Cards[k] || pe.Group != c.Group {
					t.Fatalf("set %d event %d: entity card %d (obj %d) differs", si, i, k, pe.Obj)
				}
			}
		}
	}
	if calls == 0 {
		t.Skip("no evaluation calls recorded")
	}
	t.Logf("%d calls exact; parts %+v", calls, enc.PartsStats)
}
