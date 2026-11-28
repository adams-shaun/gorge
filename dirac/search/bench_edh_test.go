package search

import (
	"encoding/binary"
	"hash/fnv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The four-seat Commander search measure: searched decisions at every seat of
// pods on the edh pool, with no net (uniform priors, the heuristic rollout
// bot). The roots are fixed by the committed slice, the tables and the seeds.

// edhSlicePath is the committed edh deck slice dirac/oracle owns.
const edhSlicePath = "../oracle/testdata/slices/edh/decks.tar.gz"

// edhPodSeats is a pod's seat count.
const edhPodSeats = 4

// edhPods4 is cardsettest.EDHPods with each pod's table released when tb ends.
func edhPods4(tb testing.TB, n int) []cardsettest.EDHPod {
	tb.Helper()
	pods := cardsettest.EDHPods(tb, edhSlicePath, n)
	tb.Cleanup(func() {
		for _, p := range pods {
			dirac.ReleaseTable(p.Table)
		}
	})
	return pods
}

// rootsEDH4 collects perSeat positions for each of the four seats from
// random-policy games on pods, game g being pod g%len(pods) under seed g. The
// wanted seat cycles 0, 1, 2, 3, so root k is seat k%4's. A root is kept when
// the decision is the wanted seat's, every seventh decision, and buildPoint
// accepts it; a game gives at most its share of the roots, so every pod
// contributes. A game that panics the kernel ("autopay found no plan" occurs on
// these pods) is abandoned and the next one starts, as BenchmarkTransitionEDH4
// does, so the roots stay a function of the pods and seeds alone.
func rootsEDH4(tb testing.TB, pods []cardsettest.EDHPod, perSeat int) []*dirac.State {
	tb.Helper()
	want := edhPodSeats * perSeat
	perGame := (want + len(pods) - 1) / len(pods)
	sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
	var out []*dirac.State
	game := func(g uint64) {
		defer func() { _ = recover() }()
		pod := &pods[g%uint64(len(pods))]
		var s dirac.State
		s.InitN(pod.Table, pod.Decks, edhPodSeats, g, dirac.Config{Mulligans: 2})
		pol := dirac.NewRandomPolicy(g)
		var d dirac.Decision
		took := 0
		for i := 0; i < 4000 && len(out) < want && took < perGame; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				return
			}
			if d.Player == uint8(len(out)%edhPodSeats) && i%7 == 3 {
				sr.root.CopyFrom(&s)
				sr.root.StepInto(&sr.rootD)
				sr.seat = d.Player
				for o := 0; o < s.NumObjs(); o++ {
					sr.seen[o] = !s.Hidden(dirac.Obj(o), d.Player)
				}
				if sr.buildPoint(&sr.root, &sr.rootD, sr.Bot.Pick(&sr.root, &sr.rootD), &sr.rootPt) {
					c := new(dirac.State)
					c.CopyFrom(&s)
					out = append(out, c)
					took++
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	for g := uint64(1); len(out) < want; g++ {
		if g > 64*uint64(len(pods)) {
			tb.Fatalf("only %d of %d edh roots after %d games", len(out), want, g-1)
		}
		game(g)
	}
	return out
}

// edhOpts is the four-seat search: the defaults with the three opponents
// searched as max^n.
func edhOpts() Options {
	o := DefaultOptions()
	o.OpponentNodes = true
	o.MaxN = true
	return o
}

func benchSearchEDH4(b *testing.B, opts Options) {
	rs := rootsEDH4(b, edhPods4(b, 8), 16)
	sims := benchSearch(b, opts, rs)
	b.ReportMetric(float64(sims)/b.Elapsed().Seconds(), "sims/s")
}

// BenchmarkSearchEDH4 searches 64 fixed edh pod roots, 16 per seat, with the
// opponents searched as max^n.
func BenchmarkSearchEDH4(b *testing.B) { benchSearchEDH4(b, edhOpts()) }

// BenchmarkSearchEDH4Paranoid is BenchmarkSearchEDH4 with MaxN off.
func BenchmarkSearchEDH4Paranoid(b *testing.B) {
	o := edhOpts()
	o.MaxN = false
	benchSearchEDH4(b, o)
}

// rootsDigest hashes every root's decision player and its ViewHash from each
// seat, and counts the roots per seat.
func rootsDigest(rs []*dirac.State) (uint64, [edhPodSeats]int) {
	h := fnv.New64a()
	var buf [8]byte
	var per [edhPodSeats]int
	for _, r := range rs {
		c := new(dirac.State)
		c.CopyFrom(r)
		var d dirac.Decision
		c.StepInto(&d)
		per[d.Player%edhPodSeats]++
		binary.LittleEndian.PutUint64(buf[:], uint64(d.Player))
		h.Write(buf[:])
		for p := uint8(0); p < edhPodSeats; p++ {
			binary.LittleEndian.PutUint64(buf[:], r.ViewHash(p, nil))
			h.Write(buf[:])
		}
	}
	return h.Sum64(), per
}

// TestSearchEDH4RootsAreFixed: two collections of the bench's roots are the
// same positions, 16 at each seat.
func TestSearchEDH4RootsAreFixed(t *testing.T) {
	pods := edhPods4(t, 8)
	a, b := rootsEDH4(t, pods, 16), rootsEDH4(t, pods, 16)
	if len(a) != 64 || len(b) != 64 {
		t.Fatalf("%d and %d roots, want 64", len(a), len(b))
	}
	da, pa := rootsDigest(a)
	db, _ := rootsDigest(b)
	if da != db {
		t.Fatalf("root digests differ: %016x, %016x", da, db)
	}
	for p, n := range pa {
		if n != 16 {
			t.Errorf("seat %d: %d roots, want 16", p, n)
		}
	}
	// The digest must see the positions: two different roots hash apart.
	d0, _ := rootsDigest(a[:1])
	d1, _ := rootsDigest(a[1:2])
	if d0 == d1 {
		t.Fatal("roots 0 and 1 have the same digest")
	}
}

// TestSearchEDH4OneSearch: one 16-simulation search on root 0 runs all 16
// simulations and answers with actions the root offers.
func TestSearchEDH4OneSearch(t *testing.T) {
	rs := rootsEDH4(t, edhPods4(t, 8), 16)
	o := edhOpts()
	o.Sims = 16
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	r := sr.Search(rs[0], 1)
	if r.Stats.Searched != 1 {
		t.Fatalf("root 0 was not searched: %+v", r.Stats)
	}
	if r.Stats.MaxN != 1 {
		t.Fatalf("root 0 did not search max^n: Stats.MaxN %d", r.Stats.MaxN)
	}
	if r.Stats.Simulations != 16 {
		t.Fatalf("Stats.Simulations %d, want 16", r.Stats.Simulations)
	}
	if r.Choice < 0 || r.Choice >= r.N || r.NActs == 0 {
		t.Fatalf("choice %d of %d candidates, %d acts", r.Choice, r.N, r.NActs)
	}
	s := new(dirac.State)
	s.CopyFrom(rs[0])
	var d dirac.Decision
	for k := 0; k < r.NActs; k++ {
		s.StepInto(&d)
		legal := false
		for i := 0; i < int(d.N); i++ {
			legal = legal || d.Options[i] == r.Acts[k]
		}
		if !legal {
			t.Fatalf("act %d (%v) is not among the decision's %d options", k, r.Acts[k], d.N)
		}
		s.Apply(r.Acts[k])
	}
}
