package search

import (
	"math"
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// attackBoard is a state at seat 0's first attackers decision with at least
// five distinct attackers (the player the only defender), and its decision.
func attackBoard(t *testing.T) (*dirac.State, dirac.Decision) {
	t.Helper()
	var a, b []dirac.CardID
	for i := 0; i < 16; i++ {
		a = append(a, v.GrizzlyBears)
	}
	for i := 0; i < 24; i++ {
		a = append(a, v.Forest)
	}
	for i := 0; i < 40; i++ {
		b = append(b, v.Mountain)
	}
	for seed := uint64(1); seed <= 32; seed++ {
		s := new(dirac.State)
		s.Init(v.Table, a, b, seed, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkAttackers {
				if d.Player == 0 && distinctSources(&d) >= 5 {
					return s, d
				}
				for j := 0; j < int(d.N); j++ {
					if d.Options[j].Kind() == dirac.AkDone {
						s.Apply(d.Options[j])
						break
					}
				}
				continue
			}
			s.Apply(d.Options[Heuristic{}.Pick(s, &d)])
		}
	}
	t.Fatal("no attackers decision with 5 creatures")
	return nil, dirac.Decision{}
}

func distinctSources(d *dirac.Decision) int {
	n, last := 0, -1
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == dirac.AkAttack && int(a.Source()) != last {
			n++
			last = int(a.Source())
		}
	}
	return n
}

func srcSetKey(acts []uint32) string {
	var l []int
	for _, a := range acts {
		if dirac.Action(a).Kind() == dirac.AkAttack {
			l = append(l, int(dirac.Action(a).Source()))
		}
	}
	sort.Ints(l)
	return string(rune(0)) + func() string {
		b := make([]byte, len(l))
		for i, x := range l {
			b[i] = byte(x)
		}
		return string(b)
	}()
}

func TestPerCreatureReachesEverySubset(t *testing.T) {
	s, d := attackBoard(t)
	n := distinctSources(&d)
	o := DefaultOptions()
	o.PerCreature = PerCreatureAttack
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	var pt point
	bot := sr.pickBot(s, &d)
	sr.attackCands(s, &d, bot, 6, &pt)
	classic := map[string]bool{}
	for i := 0; i < pt.n; i++ {
		classic[srcSetKey(pt.acts[pt.off[i]:pt.off[i+1]])] = true
	}
	if pt.n != 6 || len(classic) != 6 {
		t.Fatalf("classic point: n=%d distinct=%d, want 6", pt.n, len(classic))
	}

	reached := map[string]bool{}
	var dfs func(s *dirac.State, d dirac.Decision, c uint8, acts []uint32)
	dfs = func(s *dirac.State, d dirac.Decision, c uint8, acts []uint32) {
		var p point
		sr.chainCur = c
		bot := sr.pickBot(s, &d)
		if !sr.chainCands(s, &d, bot, MaxCands, &p) {
			t.Fatal("chainCands refused a decision offering Done")
		}
		sr.chainCur = 0
		x := sr.chainX
		var after [MaxPerm]uint8
		na := chainAfter(&d, x, &after)
		for i := 0; i < p.n; i++ {
			s2, d2 := new(dirac.State), dirac.Decision{}
			s2.CopyFrom(s)
			next := append([]uint32(nil), acts...)
			for _, a := range p.acts[p.off[i]:p.off[i+1]] {
				s2.Apply(dirac.Action(a))
				next = append(next, a)
			}
			s2.StepInto(&d2)
			nc := uint8(0)
			for _, y := range after[:na] {
				if hasAttackBy(&d2, y) {
					nc = y + 1
					break
				}
			}
			if nc == 0 {
				reached[srcSetKey(next)] = true
				continue
			}
			dfs(s2, d2, nc, next)
		}
	}
	dfs(s, d, 0, nil)

	legal := map[string]bool{}
	var kdfs func(s *dirac.State, d dirac.Decision, last int, acts []uint32)
	kdfs = func(s *dirac.State, d dirac.Decision, last int, acts []uint32) {
		if offersDone(&d) {
			legal[srcSetKey(acts)] = true
		}
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() != dirac.AkAttack || int(a.Source()) <= last {
				continue
			}
			s2, d2 := new(dirac.State), dirac.Decision{}
			s2.CopyFrom(s)
			s2.Apply(a)
			s2.StepInto(&d2)
			if d2.Kind != d.Kind || d2.Player != d.Player || s2.Turn() != s.Turn() {
				// dirac closed the declaration (a lone Done is elided)
				legal[srcSetKey(append(append([]uint32(nil), acts...), uint32(a)))] = true
				continue
			}
			kdfs(s2, d2, int(a.Source()), append(append([]uint32(nil), acts...), uint32(a)))
		}
	}
	kdfs(s, d, -1, nil)
	if len(legal) != 1<<n {
		t.Fatalf("dirac-legal sets %d, want 2^%d", len(legal), n)
	}
	if len(reached) != len(legal) {
		t.Fatalf("chain reached %d attack sets, dirac allows %d", len(reached), len(legal))
	}
	novel := 0
	for k := range reached {
		if !legal[k] {
			t.Fatal("chain reached an illegal attack set")
		}
		if !classic[k] {
			novel++
		}
	}
	if novel == 0 {
		t.Fatal("every reached set is a classic candidate")
	}
}

func perCreatureOpts() Options {
	o := DefaultOptions()
	o.PerCreature = PerCreatureAttack
	o.Sims = 16
	o.PriorTopK = 0
	return o
}

func TestPerCreatureRootDeclaration(t *testing.T) {
	s, d := attackBoard(t)
	n := distinctSources(&d)
	sr := NewSearcher(perCreatureOpts(), Uniform{}, Heuristic{})
	res := *sr.Search(s, 7)
	st := res.Stats
	if st.Searched != 1 || st.ChainSteps != n || st.Simulations != 16*n || st.WorldRoots != 0 || st.ChainFallbacks != 0 {
		t.Fatalf("stats %+v, want searched, %d steps, %d sims", st, n, 16*n)
	}
	if res.NActs < 1 || res.Acts[res.NActs-1].Kind() != dirac.AkDone {
		t.Fatalf("declaration %v does not end in AkDone", res.Acts[:res.NActs])
	}
	seen := map[uint8]bool{}
	for _, a := range res.Acts[:res.NActs-1] {
		if a.Kind() != dirac.AkAttack || seen[a.Source()] {
			t.Fatalf("bad pick %v in %v", a, res.Acts[:res.NActs])
		}
		seen[a.Source()] = true
		ok := false
		for i := 0; i < int(d.N); i++ {
			ok = ok || d.Options[i] == a
		}
		if !ok {
			t.Fatalf("pick %v was not offered", a)
		}
	}
	again := *sr.Search(s, 7)
	if again.Acts != res.Acts || again.NActs != res.NActs {
		t.Fatal("a second Search declared differently")
	}
	var seat SeatState
	if sr.SeatSearch(s, &d, 7, &seat).Record {
		t.Fatal("a per-creature declaration was recorded")
	}
	if a := testing.AllocsPerRun(3, func() { sr.Search(s, 7) }); a != 0 {
		t.Fatalf("allocs per search = %v", a)
	}
}

func TestPerCreaturePriorIsMaskedSubsetLikelihood(t *testing.T) {
	s, d := attackBoard(t)
	o := perCreatureOpts()
	o.PriorTopK = 6
	sr := NewSearcher(o, &fakeNet{val: 0.6}, Heuristic{})
	var first uint8
	zi := -1
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkAttack {
			first, zi = d.Options[i].Source(), i
			break
		}
	}
	// the only attack option of the first creature (the player defends alone)
	z := float64(float32((uint32(twoSeatCodes(d.Options[zi]))*2654435761>>7)%13) / 4)
	sr.inChain, sr.chainRoot = true, first+1
	res := sr.Search(s, 7)
	sr.inChain, sr.chainRoot = false, 0
	if res.N != 2 {
		t.Fatalf("step 0 has %d candidates, want 2", res.N)
	}
	for i := 0; i < res.N; i++ {
		if len(sr.RootCand(i)) == 1 {
			want := 1 / (1 + math.Exp(-z))
			if math.Abs(res.Prior[i]-want) > 1e-9 {
				t.Fatalf("attack prior %v, want %v", res.Prior[i], want)
			}
			return
		}
	}
	t.Fatal("no attack candidate")
}

func TestPerCreatureGame(t *testing.T) {
	run := func(seed uint64, lazy bool) []seatStep {
		o := DefaultOptions()
		o.PerCreature = PerCreatureAttack
		o.Sims = 12
		o.LazyPriors = lazy
		steps, _ := seatGame(NewSearcher(o, &fakeNet{val: 0.6}, Heuristic{}), seed)
		return steps
	}
	points, chains := 0, 0
	for seed := uint64(1); seed <= 4; seed++ {
		a, b, c := run(seed, true), run(seed, true), run(seed, false)
		if len(a) != len(b) || len(a) != len(c) {
			t.Fatalf("seed %d: game lengths differ %d %d %d", seed, len(a), len(b), len(c))
		}
		for i := range a {
			if !sameSearch(&a[i].res, &b[i].res) {
				t.Fatalf("seed %d step %d: two runs differ", seed, i)
			}
			if !sameSearch(&a[i].res, &c[i].res) {
				t.Fatalf("seed %d step %d: LazyPriors changed the search", seed, i)
			}
			st := a[i].res.Stats
			points += st.ChainPoints
			chains += st.ChainSteps
			if st.ChainFallbacks != 0 {
				t.Fatalf("seed %d step %d: %d fallbacks", seed, i, st.ChainFallbacks)
			}
		}
	}
	if points == 0 || chains == 0 {
		t.Fatalf("no chains ran: points %d steps %d", points, chains)
	}
}

func TestPerCreatureNodeCacheIsExact(t *testing.T) {
	s, _ := attackBoard(t)
	o := perCreatureOpts()
	o.CachedWorlds = 4
	plain := NewSearcher(o, Uniform{}, Heuristic{})
	o.NodeCache = 64
	cached := NewSearcher(o, Uniform{}, Heuristic{})
	for seed := uint64(1); seed <= 3; seed++ {
		a := *plain.Search(s, seed)
		b := *cached.Search(s, seed)
		if a.Visits != b.Visits || a.Choice != b.Choice || a.Q != b.Q || a.Avail != b.Avail || a.Acts != b.Acts {
			t.Fatalf("seed %d: cached search differs", seed)
		}
	}
}

// A reused Searcher declares what a fresh one does: the chain's scratch point
// carries nothing between creatures or Searches.
func TestPerCreatureIgnoresStaleScratch(t *testing.T) {
	s, _ := attackBoard(t)
	fresh := *NewSearcher(perCreatureOpts(), Uniform{}, Heuristic{}).Search(s, 5)
	sr := NewSearcher(perCreatureOpts(), Uniform{}, Heuristic{})
	sr.Search(s, 9)
	sr.chainPt.n, sr.chainPt.off[0] = 3, 0 // junk a stale point would leave behind
	got := *sr.Search(s, 5)
	if got.Acts != fresh.Acts || got.NActs != fresh.NActs {
		t.Fatalf("reused searcher declared %v, fresh %v", got.Acts[:got.NActs], fresh.Acts[:fresh.NActs])
	}
	if sr.chainPt.n > 2 {
		t.Fatalf("scratch point accumulates: n=%d", sr.chainPt.n)
	}
}

// One Searcher plays game A then game B; B's every decision equals a fresh
// Searcher's game B.
func TestPerCreatureSearcherReuseIsExact(t *testing.T) {
	mk := func() *Searcher {
		o := DefaultOptions()
		// a host's search seat defaults
		o.Kinds, o.PriorTopK, o.OpponentNodes, o.OpponentMirror, o.ClosedList = AllKinds(), 0, false, false, false
		o.PerCreature = 3
		o.Sims = 40
		o.CPUCT, o.OpponentNodes, o.OpponentMirror, o.Kinds, o.Clairvoyant = 1, true, true, FullKinds(), true
		return NewSearcher(o, &fakeNet{val: 0.6}, Heuristic{})
	}
	pts := 0
	defer func() { t.Logf("chain points %d", pts) }()
	for seedA := uint64(1); seedA <= 4; seedA++ {
		for seedB := uint64(1); seedB <= 4; seedB++ {
			if seedA == seedB {
				continue
			}
			reused := mk()
			seatGame(reused, seedA)
			for o := range reused.seen { // what a game with more objects left behind
				reused.seen[o] = true
			}
			got, _ := seatGame(reused, seedB)
			want, _ := seatGame(mk(), seedB)
			if len(got) != len(want) {
				t.Fatalf("A=%d B=%d: %d decisions reused, %d fresh", seedA, seedB, len(got), len(want))
			}
			for i := range got {
				pts += got[i].res.Stats.ChainPoints
				if !sameSearch(&got[i].res, &want[i].res) {
					t.Fatalf("A=%d B=%d step %d: reused searcher differs from fresh", seedA, seedB, i)
				}
			}
		}
	}
}

// Objects past the root's (a simulation's tokens) are unseen: a Search
// clears what an earlier, larger root marked seen there, so their keys are
// the same on a reused Searcher as on a fresh one.
func TestPerCreatureSeenIgnoresStaleObjects(t *testing.T) {
	s, _ := attackBoard(t)
	sr := NewSearcher(perCreatureOpts(), Uniform{}, Heuristic{})
	for o := range sr.seen {
		sr.seen[o] = true
	}
	sr.Search(s, 5)
	for o := s.NumObjs(); o < len(sr.seen); o++ {
		if sr.seen[o] {
			t.Fatalf("object %d past the root's %d objects is marked seen", o, s.NumObjs())
		}
	}
}
