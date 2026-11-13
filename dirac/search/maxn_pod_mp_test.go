//go:build dirac_mp

package search

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// podDigest plays a four-seat vanilla pod from seed for at most steps
// decisions, every seat answering with sr.Search, and hashes every searched
// result (candidates, visits, values, prior, choice). It returns the digest,
// the searches run and how many of them ran max^n (Stats.MaxN).
func podDigest(sr *Searcher, seed uint64, steps int) (digest uint64, searched, maxN int) {
	h := fnv.New64a()
	var buf [8]byte
	put := func(x uint64) {
		binary.LittleEndian.PutUint64(buf[:], x)
		h.Write(buf[:])
	}
	var s dirac.State
	var decks [dirac.MaxSeats][]dirac.CardID
	for p := range decks {
		decks[p] = v.Decks[(int(seed)+p)%len(v.Decks)]
	}
	s.InitN(v.Table, decks, 4, seed, dirac.Config{})
	var d dirac.Decision
	for i := 0; i < steps; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		r := sr.Search(&s, uint64(i)*0x51+seed)
		if r.Stats.Searched > 0 {
			searched++
			maxN += r.Stats.MaxN
			put(uint64(d.Player))
			put(uint64(r.N))
			put(uint64(r.Choice))
			for j := 0; j < r.N; j++ {
				put(uint64(uint32(r.Visits[j])))
				put(math.Float64bits(r.Q[j]))
				put(math.Float64bits(r.Prior[j]))
			}
			put(math.Float64bits(r.RootValue))
		}
		r.Play(&s)
	}
	put(uint64(uint8(s.Winner())))
	return h.Sum64(), searched, maxN
}

// podOpts is the pinned pod search: the default options (the opponents in
// the tree, mirrored) at 24 simulations.
func podOpts() Options {
	o := DefaultOptions()
	o.Sims = 24
	return o
}

// TestMaxNOffIsBitIdentical pins four-seat pod searches with MaxN off to the
// digests they hashed to on the commit before MaxN existed, under the
// heuristic leaf (a VecEvaluator) and the fake network (split vectors); the
// option off allocates no vector.
func TestMaxNOffIsBitIdentical(t *testing.T) {
	for _, tc := range []struct {
		name string
		ev   Evaluator
		want uint64
	}{
		{"heur", HeuristicLeaf{}, 0x8dc30a36801cbc50},
		{"net", &fakeNet{val: -1}, 0x6866394ca354eec0},
	} {
		sr := NewSearcher(podOpts(), tc.ev, Heuristic{})
		if sr.t.nwv != nil || sr.t.ewv != nil {
			t.Fatalf("%s: MaxN off allocated the value vectors", tc.name)
		}
		got, searched, maxN := podDigest(sr, 3, 600)
		if searched == 0 || maxN != 0 {
			t.Fatalf("%s: %d searches, %d under max^n", tc.name, searched, maxN)
		}
		if got != tc.want {
			t.Errorf("%s: digest %016x, want %016x (%d searches)", tc.name, got, tc.want, searched)
		}
	}
}

// TestMaxNSearchesFourSeatPods: with MaxN on, every four-seat search with
// more than two living seats runs max^n, the pod plays differently from the
// paranoid search, the same seed replays it exactly, and a search allocates
// nothing.
func TestMaxNSearchesFourSeatPods(t *testing.T) {
	for _, ev := range []Evaluator{HeuristicLeaf{}, &fakeNet{val: -1}} {
		off, _, _ := podDigest(NewSearcher(podOpts(), ev, Heuristic{}), 3, 600)
		o := podOpts()
		o.MaxN = true
		on, searched, maxN := podDigest(NewSearcher(o, ev, Heuristic{}), 3, 600)
		if searched == 0 || maxN == 0 {
			t.Fatalf("%T: %d searches, %d under max^n", ev, searched, maxN)
		}
		if on == off {
			t.Fatalf("%T: max^n hashed as the paranoid search", ev)
		}
		if again, _, _ := podDigest(NewSearcher(o, ev, Heuristic{}), 3, 600); again != on {
			t.Fatalf("%T: max^n replayed to %016x, first %016x", ev, again, on)
		}
		t.Logf("%T: %d searches, %d max^n", ev, searched, maxN)
	}
	o := podOpts()
	o.MaxN = true
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	var s dirac.State
	var decks [dirac.MaxSeats][]dirac.CardID
	for p := range decks {
		decks[p] = v.Decks[p%len(v.Decks)]
	}
	s.InitN(v.Table, decks, 4, 11, dirac.Config{})
	var d dirac.Decision
	for i := 0; i < 2000; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			t.Fatal("no searched max^n position")
		}
		r := sr.Search(&s, 7)
		if r.Stats.Searched > 0 && r.Stats.MaxN == 1 && r.Stats.OppPoints > 0 {
			if a := testing.AllocsPerRun(3, func() { sr.Search(&s, 7) }); a != 0 {
				t.Fatalf("allocs per max^n search = %v", a)
			}
			return
		}
		r.Play(&s)
	}
	t.Fatal("no searched max^n position with an opponent point")
}

// TestHeuristicLeafValueVec: the living seats' entries sum to 1, a lost
// seat's is 0, entries follow turn order from the seat, and each is the
// softmax of the scores Value reads.
func TestHeuristicLeafValueVec(t *testing.T) {
	var s dirac.State
	var decks [dirac.MaxSeats][]dirac.CardID
	for p := range decks {
		decks[p] = v.Decks[(p+1)%len(v.Decks)]
	}
	s.InitN(v.Table, decks, 4, 13, dirac.Config{})
	pol := dirac.NewRandomPolicy(6)
	var d dirac.Decision
	checked := 0
	for i := 0; i < 4000 && !s.Over(); i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if i%40 == 0 {
			score := heuristicScores(&s)
			for seat := uint8(0); seat < 4; seat++ {
				var vec [dirac.MaxSeats]float64
				HeuristicLeaf{}.ValueVec(&s, &d, seat, &vec)
				sum, z := 0.0, 0.0
				for p := uint8(0); p < 4; p++ {
					if !s.FeatLost(p) {
						z += math.Exp(score[p] / 20)
					}
				}
				for p := uint8(0); p < 4; p++ {
					r := relSeat(p, seat, 4)
					sum += vec[r]
					want := 0.0
					if !s.FeatLost(p) {
						want = math.Exp(score[p]/20) / z
					}
					if math.Abs(vec[r]-want) > 1e-12 {
						t.Fatalf("step %d seat %d: entry %d (seat %d) %v, want %v", i, seat, r, p, vec[r], want)
					}
				}
				if math.Abs(sum-1) > 1e-12 {
					t.Fatalf("step %d seat %d: entries sum to %v", i, seat, sum)
				}
				checked++
			}
		}
		s.Apply(pol.Pick(&d))
	}
	if checked == 0 {
		t.Fatal("no position checked")
	}
}
