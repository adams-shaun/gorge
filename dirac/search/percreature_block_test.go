package search

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// menaceBoard is a state at seat 1's blockers decision with exactly one
// Boggart Brute (menace) attacking and k Grizzly Bears offered as blockers.
func menaceBoard(t *testing.T, k int) (*dirac.State, dirac.Decision) {
	t.Helper()
	var a, b []dirac.CardID
	for i := 0; i < 16; i++ {
		a = append(a, v.BoggartBrute)
		b = append(b, v.GrizzlyBears)
	}
	for i := 0; i < 24; i++ {
		a = append(a, v.Mountain)
		b = append(b, v.Forest)
	}
	for seed := uint64(1); seed <= 32; seed++ {
		s := new(dirac.State)
		s.Init(v.Table, a, b, seed, dirac.Config{})
		var d dirac.Decision
		attacked := false
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkBlockers && d.Player == 1 {
				srcs, tgts := map[uint8]bool{}, map[uint8]bool{}
				for j := 0; j < int(d.N); j++ {
					if o := d.Options[j]; o.Kind() == dirac.AkBlock {
						srcs[uint8(o.Source())], tgts[uint8(o.Target())] = true, true
					}
				}
				if len(srcs) == k && len(tgts) == 1 && offersDone(&d) {
					return s, d
				}
			}
			if d.Kind == dirac.DkAttackers {
				pick := -1
				for j := 0; j < int(d.N); j++ {
					o := d.Options[j]
					if o.Kind() == dirac.AkDone {
						pick = j
					}
				}
				if d.Player == 0 && !attacked {
					bears := 0
					for o := 0; o < s.NumObjs(); o++ {
						if s.Controller(dirac.Obj(o)) == 1 && s.IsCreature(dirac.Obj(o)) && !s.Tapped(dirac.Obj(o)) {
							bears++
						}
					}
					if bears >= k {
						for j := 0; j < int(d.N); j++ {
							if d.Options[j].Kind() == dirac.AkAttack {
								pick, attacked = j, true
								break
							}
						}
					}
				}
				s.Apply(d.Options[pick])
				continue
			}
			if d.Kind == dirac.DkBlockers {
				attacked = false
			}
			s.Apply(d.Options[Heuristic{}.Pick(s, &d)])
		}
	}
	t.Fatalf("no blockers decision with one Brute and %d bears", k)
	return nil, dirac.Decision{}
}

func blockSetKey(acts []uint32) string {
	var l []int
	for _, a := range acts {
		if dirac.Action(a).Kind() == dirac.AkBlock {
			l = append(l, int(dirac.Action(a).Source()))
		}
	}
	sort.Ints(l)
	b := make([]byte, len(l))
	for i, x := range l {
		b[i] = byte(x)
	}
	return string(b)
}

func blockOpts() Options {
	o := DefaultOptions()
	o.PerCreature = PerCreatureBlock
	return o
}

// chainBlockSets walks the block chain's DFS exactly as chainPlay does and
// returns the declarations it reaches (by blocker set); every chain end must
// offer AkDone (or have been closed by dirac).
func chainBlockSets(t *testing.T, sr *Searcher, s *dirac.State, d dirac.Decision) map[string]bool {
	t.Helper()
	reached := map[string]bool{}
	turn0 := s.Turn()
	var dfs func(s *dirac.State, d dirac.Decision, c uint8, acts []uint32)
	end := func(s *dirac.State, d dirac.Decision, acts []uint32) {
		d2 := dirac.Decision{}
		s.StepInto(&d2)
		if d2.Kind == d.Kind && d2.Player == d.Player && s.Turn() == turn0 {
			// open: the declaration must be closable and legal
			if !offersDone(&d2) {
				t.Fatalf("chain end %v withholds AkDone", blockSetKey(acts))
			}
			if len(blockSetKey(acts)) == 1 {
				t.Fatalf("chain end %v: a lone blocker on a menace attacker", blockSetKey(acts))
			}
		}
		reached[blockSetKey(acts)] = true
	}
	dfs = func(s *dirac.State, d dirac.Decision, c uint8, acts []uint32) {
		var p point
		sr.chainCur, sr.nForced = c, 0
		bot := sr.pickBot(s, &d)
		if !sr.chainCands(s, &d, bot, MaxCands, &p) {
			t.Fatal("chainCands refused a decision offering Done")
		}
		sr.chainCur = 0
		if p.n == 0 {
			next := append([]uint32(nil), acts...)
			for _, a := range sr.chainForced[:sr.nForced] {
				s.Apply(dirac.Action(a))
				next = append(next, a)
			}
			end(s, d, next)
			return
		}
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
			if d2.Kind == d.Kind && d2.Player == d.Player {
				for _, y := range after[:na] {
					if hasAttackBy(&d2, y) {
						nc = y + 1
						break
					}
				}
			}
			if nc == 0 {
				end(s2, d, next)
				continue
			}
			dfs(s2, d2, nc, next)
		}
	}
	s0 := new(dirac.State)
	s0.CopyFrom(s)
	dfs(s0, d, 0, nil)
	return reached
}

// legalBlockSets is dirac's own DFS: every blocker set recorded where
// AkDone is offered (or dirac closed the declaration).
func legalBlockSets(s *dirac.State, d dirac.Decision) map[string]bool {
	legal := map[string]bool{}
	var kdfs func(s *dirac.State, d dirac.Decision, last int, acts []uint32)
	kdfs = func(s *dirac.State, d dirac.Decision, last int, acts []uint32) {
		if offersDone(&d) {
			legal[blockSetKey(acts)] = true
		}
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			if a.Kind() != dirac.AkBlock || int(a.Source()) <= last {
				continue
			}
			s2, d2 := new(dirac.State), dirac.Decision{}
			s2.CopyFrom(s)
			s2.Apply(a)
			s2.StepInto(&d2)
			next := append(append([]uint32(nil), acts...), uint32(a))
			if d2.Kind != d.Kind || d2.Player != d.Player || s2.Turn() != s.Turn() {
				legal[blockSetKey(next)] = true
				continue
			}
			kdfs(s2, d2, int(a.Source()), next)
		}
	}
	kdfs(s, d, -1, nil)
	return legal
}

func TestPerCreatureMenaceBlocks(t *testing.T) {
	// k = 2: only no block, or both bears on the Brute
	s, d := menaceBoard(t, 2)
	sr := NewSearcher(blockOpts(), Uniform{}, Heuristic{})
	got := chainBlockSets(t, sr, s, d)
	// {B1} is B1 declared with dirac playing the forced B2 and Done
	// itself (it closes the declaration on a lone option)
	legal2 := legalBlockSets(s, d)
	if len(got) != 2 || !got[""] {
		t.Fatalf("k=2 reached %d sets %v, dirac %v, want {} and {B1,B2}", len(got), got, legal2)
	}
	for k := range got {
		if !legal2[k] {
			t.Fatalf("k=2 reached %q, not dirac-legal", k)
		}
	}
	// B2's point after B1 blocks / skips
	var first, second uint8
	var br dirac.Action
	n := 0
	for i := 0; i < int(d.N); i++ {
		if o := d.Options[i]; o.Kind() == dirac.AkBlock {
			if n == 0 {
				first, br = uint8(o.Source()), o
			} else if o.Source() != dirac.TCode(first) {
				second = uint8(o.Source())
				break
			}
			n++
		}
	}
	var p point
	sr.chainCur, sr.nForced = second+1, 0
	// dirac plays B2 itself once B1 has blocked, so ask before it steps
	var s1 dirac.State
	s1.CopyFrom(s)
	s1.Apply(br)
	sr.chainCands(&s1, &d, 0, MaxCands, &p)
	if p.n != 0 || sr.nForced != 1 {
		t.Fatalf("after B1 blocks: n=%d forced=%d, want 0 and 1", p.n, sr.nForced)
	}
	p.n, sr.nForced = 0, 0
	sr.chainCands(s, &d, 0, MaxCands, &p)
	if p.n != 0 || sr.nForced != 0 {
		t.Fatalf("after B1 skips: n=%d forced=%d, want 0 and 0", p.n, sr.nForced)
	}
	sr.chainCur = 0

	// k = 3: the chain reaches exactly dirac-legal declarations
	s, d = menaceBoard(t, 3)
	got = chainBlockSets(t, sr, s, d)
	legal := legalBlockSets(s, d)
	if len(legal) != 5 || len(got) != len(legal) {
		t.Fatalf("k=3: chain reached %d sets, dirac allows %d (want 5)", len(got), len(legal))
	}
	for k := range got {
		if !legal[k] || len(k) == 1 {
			t.Fatalf("k=3: bad reached set %q", k)
		}
	}
}

func TestPerCreatureBlockRoot(t *testing.T) {
	s, _ := menaceBoard(t, 3)
	o := blockOpts()
	o.Sims = 16
	o.PriorTopK = 0
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	res := *sr.Search(s, 7)
	if res.Stats.ChainFallbacks != 0 {
		t.Fatalf("%d fallbacks", res.Stats.ChainFallbacks)
	}
	if res.NActs < 1 || res.Acts[res.NActs-1].Kind() != dirac.AkDone {
		t.Fatalf("declaration %v does not end in AkDone", res.Acts[:res.NActs])
	}
	var dd dirac.Decision
	blockers := 0
	for _, a := range res.Acts[:res.NActs] {
		s.StepInto(&dd)
		if dd.Kind != dirac.DkBlockers {
			break // closed by dirac
		}
		if !Offered(s, a, dirac.DkBlockers, dd.Player, &dd) {
			t.Fatalf("act %v of %v was not offered", a, res.Acts[:res.NActs])
		}
		if a.Kind() == dirac.AkBlock {
			blockers++
		}
		s.Apply(a)
	}
	if blockers == 1 {
		t.Fatal("a lone blocker on a menace attacker")
	}
	s.StepInto(&dd)
	if dd.Kind == dirac.DkBlockers {
		t.Fatal("the declaration is still open")
	}
}

func TestPerCreatureBothGame(t *testing.T) {
	run := func(seed uint64, lazy bool) []seatStep {
		o := DefaultOptions()
		o.PerCreature = PerCreatureAttack | PerCreatureBlock
		o.Sims = 12
		o.LazyPriors = lazy
		steps, _ := seatGame(NewSearcher(o, &fakeNet{val: 0.6}, Heuristic{}), seed)
		return steps
	}
	points := 0
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
			if f := a[i].res.Stats.ChainFallbacks; f != 0 {
				t.Fatalf("seed %d step %d: %d fallbacks", seed, i, f)
			}
			points += a[i].res.Stats.ChainPoints
		}
	}
	if points == 0 {
		t.Fatal("no chains ran")
	}
}
