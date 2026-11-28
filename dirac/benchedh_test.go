package dirac

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The Commander-pod speed measure. The gate's FDN and BLB benches are two-seat
// 40-card games; the pods the program is mandated to play are four seats of
// 100-card decks on the edh pool, the largest State. These benches play exactly
// that, so a change that costs a pod (a per-seat loop, a per-object walk over
// 400 cards) shows here and not in the two-deck benches.

// edhSlicePath is the committed edh deck slice, the 52 MTGJSON precons the edh
// pool's pairings are cut from. dirac/oracle owns it, and imports this package,
// so the bench reads the tarball directly.
const edhSlicePath = "oracle/testdata/slices/edh/decks.tar.gz"

// edhDecks returns the committed edh decks, 100 cards each, sorted by deck name.
func edhDecks(tb testing.TB) [][]cardset.Entry {
	tb.Helper()
	f, err := os.Open(edhSlicePath)
	if err != nil {
		tb.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		tb.Fatal(err)
	}
	byName := map[string][]cardset.Entry{}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			tb.Fatal(err)
		}
		if !strings.HasPrefix(h.Name, "decks/") || !strings.HasSuffix(h.Name, ".json") {
			continue
		}
		var d fdnDeck
		if err := json.NewDecoder(tr).Decode(&d); err != nil {
			tb.Fatalf("%s: %v", h.Name, err)
		}
		var es []cardset.Entry
		size := 0
		for _, c := range d.Cards {
			es = append(es, cardset.Entry{Name: c.Name, Count: c.Count})
			size += c.Count
		}
		if size != 100 {
			tb.Fatalf("%s: %d cards, want 100", h.Name, size)
		}
		byName[h.Name] = es
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([][]cardset.Entry, len(names))
	for i, n := range names {
		out[i] = byName[n]
	}
	if len(out) < MaxSeats {
		tb.Fatalf("%d edh decks, a pod needs %d", len(out), MaxSeats)
	}
	return out
}

// edhSet is the edh pool as kfuzz -allow-unverified plays it (no edh card has
// evidence of its own yet), skipping with no corpus or tables as the oracle's
// edh tests do.
func edhSet(tb testing.TB) cardset.Set {
	tb.Helper()
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		tb.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		tb.Fatal(err)
	}
	return cardset.AllowUnverified(set)
}

type edhPod struct {
	tbl   *Table
	decks [MaxSeats][]CardID
}

// edhPods builds n four-seat pods: candidate c is the decks c, c+7, c+14 and
// c+21 of the sorted slice (wrapping), so no two candidates share a deck set
// below 52. A pod the kernel refuses for a card moves to the next candidate, so
// the pods are fixed by the committed slice and the tables alone.
func edhPods(tb testing.TB, n int) []edhPod {
	tb.Helper()
	decks := edhDecks(tb)
	set := edhSet(tb)
	out := make([]edhPod, 0, n)
	tb.Cleanup(func() {
		for _, p := range out {
			ReleaseTable(p.tbl)
		}
	})
	for c := 0; len(out) < n; c++ {
		if c >= len(decks) {
			tb.Fatalf("only %d of %d edh pods build", len(out), n)
		}
		pick := make([][]cardset.Entry, MaxSeats)
		for j := range pick {
			pick[j] = decks[(c+7*j)%len(decks)]
		}
		tbl, ids, err := set.MatchupN(pick)
		if edhPodRefused(err) {
			continue
		}
		if err != nil {
			tb.Fatal(err)
		}
		p := edhPod{tbl: tbl}
		copy(p.decks[:], ids)
		out = append(out, p)
	}
	return out
}

// edhPodRefused is a refusal by name or by a card interaction the kernel guards
// against: that pod is skipped, any other error fails the bench.
func edhPodRefused(err error) bool {
	var re *cardset.RefusedError
	var ir *cardset.InteractionRefusal
	if errors.As(err, &re) || errors.As(err, &ir) {
		return true
	}
	if err == nil {
		return false
	}
	for _, p := range []string{"layer guard: ", "Room name read: ", "ReduceCost Type Ability beside "} {
		if strings.HasPrefix(err.Error(), p) {
			return true
		}
	}
	return false
}

func (p *edhPod) init(s *State, seed uint64) {
	p.initCfg(s, seed, Config{Mulligans: 2})
}

func (p *edhPod) initCfg(s *State, seed uint64, cfg Config) {
	s.InitN(p.tbl, p.decks, MaxSeats, seed, cfg)
}

// edhAutopayConfig is LockstepConfig's auto-pay surface (the one dpod, kpod
// and the search play) with the EDH bench's two mulligans.
func edhAutopayConfig() Config {
	cfg := LockstepConfig(SurfaceAutopay)
	cfg.Mulligans = 2
	return cfg
}

// BenchmarkTransitionEDH4 plays 16 fixed four-seat edh pods under
// NewRandomPolicy, game g being pod g%16 under seed g+1, a pod restarting at
// game over as BenchmarkTransitionFDN does. It allocates nothing per
// transition once each pod has played a game.
//
// A random-policy edh game can panic the kernel ("autopay found no plan", about
// 4% of games on these pods); the bench abandons that game and starts the next,
// so the measure stays a function of the pods alone. The recover is per game
// segment, not per transition.
func BenchmarkTransitionEDH4(b *testing.B) {
	benchTransitionEDH4(b, Config{Mulligans: 2})
}

// BenchmarkTransitionEDH4Autopay is BenchmarkTransitionEDH4's pods, seeds and
// loop on LockstepConfig's auto-pay surface (edhAutopayConfig): the option
// surface the search plays, whose script walk prices plays the printed
// sources pay (scriptOffers' real walk).
func BenchmarkTransitionEDH4Autopay(b *testing.B) {
	benchTransitionEDH4(b, edhAutopayConfig())
}

func benchTransitionEDH4(b *testing.B, cfg Config) {
	pods := edhPods(b, 16)
	var s State
	var d Decision
	g, i, panics := 0, 0, 0
	var pol RandomPolicy
	start := func() {
		pods[g%len(pods)].initCfg(&s, uint64(g)+1, cfg)
		pol = NewRandomPolicy(uint64(g) + 1)
	}
	segment := func() {
		defer func() {
			if recover() != nil {
				panics++
				g++
				start()
			}
		}()
		for ; i < b.N; i++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				start()
				s.StepInto(&d)
			}
			s.Apply(pol.Pick(&d))
		}
	}
	// Each pod plays one game untimed: a table's first InitN allocates its
	// cold arrays (about 3.7 MB) and a few paths allocate on first use.
	start()
	for g < len(pods) {
		func() {
			defer func() {
				if recover() != nil {
					g++
					start()
				}
			}()
			for g < len(pods) {
				s.StepInto(&d)
				if d.Kind == DkNone {
					g++
					start()
					continue
				}
				s.Apply(pol.Pick(&d))
			}
		}()
	}
	g = 0
	start()
	b.ReportAllocs()
	var m0, m1 runtime.MemStats
	runtime.ReadMemStats(&m0)
	b.ResetTimer()
	for i < b.N {
		segment()
		if i < b.N {
			i++ // the segment ended in a panic: that transition still counts
		}
	}
	b.StopTimer()
	runtime.ReadMemStats(&m1)
	if per := (m1.Mallocs - m0.Mallocs) / uint64(b.N); per != 0 {
		b.Fatalf("%d allocs per transition, want 0", per)
	}
	if panics > 0 {
		b.Logf("%d of %d games ended in a kernel panic", panics, g+1)
	}
}

// BenchmarkCloneEDH4 copies a mid-game four-seat edh State: the first State of
// pod 1 with 40 battlefield slots in use, else the first at turn 12.
func BenchmarkCloneEDH4(b *testing.B) {
	pod := edhPods(b, 1)[0]
	var s, mid State
	pod.init(&s, 1)
	pol := NewRandomPolicy(1)
	var d Decision
	have := false
	for s.permHi < 40 {
		s.StepInto(&d)
		if d.Kind == DkNone {
			break
		}
		s.Apply(pol.Pick(&d))
		if !have && s.turn >= 12 {
			mid.CopyFrom(&s)
			have = true
		}
	}
	if s.permHi < 40 {
		if !have {
			b.Fatal("the pod ended before turn 12 or 40 permanents")
		}
		s.CopyFrom(&mid)
	}
	b.Logf("cloning a State at turn %d, permHi %d, nobj %d", s.turn, s.permHi, s.nobj)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cloneDst.CopyFrom(&s)
	}
}

// TestEDHBenchDeterministic: two runs of the first 2000 transitions of pod 0
// (restarting at game over, game g under seed g+1, as the bench does) reach the
// same Projection and the same number of games, so the bench is not silently
// nondeterministic.
func TestEDHBenchDeterministic(t *testing.T) {
	pod := edhPods(t, 1)[0]
	run := func() (*Projection, int) {
		var s State
		var d Decision
		g := 0
		pod.init(&s, 1)
		pol := NewRandomPolicy(1)
		for n := 0; n < 2000; n++ {
			s.StepInto(&d)
			if d.Kind == DkNone {
				g++
				pod.init(&s, uint64(g)+1)
				pol = NewRandomPolicy(uint64(g) + 1)
				s.StepInto(&d)
			}
			s.Apply(pol.Pick(&d))
		}
		if g == 0 || s.nobj == 0 {
			t.Fatalf("%d games, %d objects: nothing was exercised", g, s.nobj)
		}
		p := new(Projection)
		s.Project(p)
		t.Logf("%d games restarted, last at turn %d, nobj %d", g, s.turn, s.nobj)
		return p, g
	}
	a, ga := run()
	b, gb := run()
	if ga != gb || *a != *b {
		t.Fatalf("two runs of pod 0 differ (games %d, %d; projections equal: %v)", ga, gb, *a == *b)
	}
}
