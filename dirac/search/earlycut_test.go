package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEarlyCutGapKeepsThePick: where the gap rule stops a search (the
// runner-up cannot catch the leader), the pick is the full search's, the
// visits are a prefix of it (no visit beyond the full count) and fewer
// simulations ran; the share rule stops after its minimum.
func TestEarlyCutGapKeepsThePick(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 200
	full := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	g := o
	g.EarlyCutGap = true
	gap := NewSearcher(g, HeuristicLeaf{}, Heuristic{})
	sh := o
	sh.EarlyCutShare, sh.EarlyCutMin = 0.6, 40
	share := NewSearcher(sh, HeuristicLeaf{}, Heuristic{})
	rs := roots(60)
	cuts, shareCuts, saved := 0, 0, 0
	for i := range rs {
		a := *full.Search(rs[i], uint64(i))
		b := *gap.Search(rs[i], uint64(i))
		if a.Stats.EarlyCut != 0 || a.Stats.Simulations != o.Sims {
			t.Fatalf("root %d: the uncut search cut (%d sims)", i, a.Stats.Simulations)
		}
		if b.Choice != a.Choice || b.Acts != a.Acts {
			t.Fatalf("root %d: gap cut changed the pick: %d vs %d (visits %v vs %v)", i, b.Choice, a.Choice, b.Visits[:b.N], a.Visits[:a.N])
		}
		for j := 0; j < a.N; j++ {
			if b.Visits[j] > a.Visits[j] {
				t.Fatalf("root %d: cut visits %v exceed full %v", i, b.Visits[:b.N], a.Visits[:a.N])
			}
		}
		if b.Stats.EarlyCut != 0 {
			cuts++
			saved += o.Sims - b.Stats.Simulations
			if b.Stats.Simulations >= o.Sims {
				t.Fatalf("root %d: cut but ran %d sims", i, b.Stats.Simulations)
			}
		} else if b.Visits != a.Visits {
			t.Fatalf("root %d: uncut gap search differs from the full one", i)
		}
		c := *share.Search(rs[i], uint64(i))
		if c.Stats.EarlyCut != 0 {
			shareCuts++
			if c.Stats.Simulations < sh.EarlyCutMin {
				t.Fatalf("root %d: share cut after %d < %d sims", i, c.Stats.Simulations, sh.EarlyCutMin)
			}
		}
	}
	if cuts == 0 || shareCuts == 0 {
		t.Fatalf("%d of %d roots gap-cut, %d share-cut", cuts, len(rs), shareCuts)
	}
	t.Logf("%d of %d roots gap-cut, %d sims saved; %d share-cut", cuts, len(rs), saved, shareCuts)
}

// TestEarlyCutGapGameUnchanged: a game whose searching seat gap-cuts is the
// uncut game move for move.
func TestEarlyCutGapGameUnchanged(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 100
	g := o
	g.EarlyCutGap = true
	for seed := uint64(1); seed <= 3; seed++ {
		w1, n1 := playVsRandom(NewSearcher(o, HeuristicLeaf{}, Heuristic{}), seed, 0, [2]int{int(seed % 5), int(seed+1) % 5})
		w2, n2 := playVsRandom(NewSearcher(g, HeuristicLeaf{}, Heuristic{}), seed, 0, [2]int{int(seed % 5), int(seed+1) % 5})
		if w1 != w2 || n1 != n2 {
			t.Fatalf("seed %d: gap cut changed the game: winner %d/%d, searched %d/%d", seed, w1, w2, n1, n2)
		}
	}
}

// TestCutTraceSpansPerDecisionBudgets: with a per-decision budget
// (SimsByKind) the trace is sized to MaxSims and records the whole of each
// decision's search (N = Result.Sims), so -cut-trace can score every
// searched decision against its own budget.
func TestCutTraceSpansPerDecisionBudgets(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 4
	o.SimsByKind[kindPriority] = 40
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	sr.Trace = NewCutTrace(MaxSims(&o))
	if len(sr.Trace.Lead) != 40 {
		t.Fatalf("trace sized %d, want MaxSims 40", len(sr.Trace.Lead))
	}
	kinds := map[bool]int{}
	for seed := uint64(1); seed <= 4; seed++ {
		steps, _ := seatGame(sr, seed)
		// the trace holds the last search only
		last := -1
		for i := range steps {
			if steps[i].res.Stats.Searched != 0 {
				last = i
			}
			r := &steps[i].res
			if r.Stats.Searched == 0 {
				continue
			}
			prio := r.Kind == kindPriority
			want := 4
			if prio {
				want = 40
			}
			if r.Sims != want || r.Stats.Simulations != want {
				t.Fatalf("seed %d: a %v search ran %d of %d simulations", seed, prio, r.Stats.Simulations, want)
			}
			kinds[prio]++
		}
		if last < 0 {
			t.Fatalf("seed %d: no searched decision", seed)
		}
		r := &steps[last].res
		want := 4
		if r.Kind == kindPriority {
			want = 40
		}
		if sr.Trace.N != want {
			t.Fatalf("seed %d: the trace holds %d entries of the %d-budget search", seed, sr.Trace.N, want)
		}
	}
	if kinds[true] == 0 || kinds[false] == 0 {
		t.Fatalf("searched priority %d, other %d: the games never reached both budgets", kinds[true], kinds[false])
	}
}

func TestEarlyCutZeroAllocs(t *testing.T) {
	o := DefaultOptions()
	o.EarlyCutGap, o.EarlyCutShare, o.EarlyCutMin = true, 0.6, 20
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 11, dirac.Config{})
	pol := dirac.NewRandomPolicy(5)
	var d dirac.Decision
	for i := 0; i < 60; i++ {
		s.StepInto(&d)
		s.Apply(pol.Pick(&d))
	}
	for {
		s.StepInto(&d)
		if d.Player == 0 && d.Kind == dirac.DkPriority {
			break
		}
		s.Apply(pol.Pick(&d))
	}
	if allocs := testing.AllocsPerRun(5, func() { sr.Search(&s, 1) }); allocs != 0 {
		t.Fatalf("allocs per search = %v", allocs)
	}
}

// TestCutTraceIsThePrefix: a full search's trace predicts every cut
// search exactly -- where the rule fires, the simulations it runs and its
// pick -- so rules can be scored offline from one full search.
func TestCutTraceIsThePrefix(t *testing.T) {
	o := DefaultOptions()
	o.Sims = 200
	full := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	full.Trace = NewCutTrace(o.Sims)
	rules := []CutRule{{Gap: true}, {Share: 0.6, Min: 40}, {Share: 0.5, Min: 10}, {Gap: true, Share: 0.8, Min: 100}}
	cut := make([]*Searcher, len(rules))
	for j, r := range rules {
		c := o
		c.EarlyCutGap, c.EarlyCutShare, c.EarlyCutMin = r.Gap, r.Share, r.Min
		cut[j] = NewSearcher(c, HeuristicLeaf{}, Heuristic{})
	}
	rs := roots(40)
	fired := 0
	for i := range rs {
		a := *full.Search(rs[i], uint64(i))
		tr := full.Trace
		if tr.N != o.Sims || int(tr.Arg[tr.N-1]) != a.Choice {
			t.Fatalf("root %d: trace N %d, final arg %d vs choice %d", i, tr.N, tr.Arg[tr.N-1], a.Choice)
		}
		for j, r := range rules {
			at := o.Sims
			for k := 1; k < o.Sims; k++ {
				if r.Fires(tr.Lead[k-1], tr.Run[k-1], tr.Total[k-1], k, o.Sims) {
					at = k
					break
				}
			}
			b := *cut[j].Search(rs[i], uint64(i))
			if b.Stats.Simulations != at || b.Choice != int(tr.Arg[at-1]) {
				t.Fatalf("root %d rule %+v: cut search ran %d sims, picked %d; trace says %d, %d", i, r, b.Stats.Simulations, b.Choice, at, tr.Arg[at-1])
			}
			if at < o.Sims {
				fired++
			}
		}
	}
	if fired == 0 {
		t.Fatal("no rule fired")
	}
}

// TestClairvoyantDealsTheRealOpponent: under Clairvoyant every world keeps
// the opponent's real hand and the multiset of its real library; without
// it the closed/open-list deal changes the hand.
func TestClairvoyantDealsTheRealOpponent(t *testing.T) {
	rs := roots(20)
	var real, got [2 * dirac.MaxDeck]dirac.CardID
	for _, cv := range []bool{true, false} {
		o := DefaultOptions()
		o.Clairvoyant = cv
		sr := NewSearcher(o, Uniform{}, Heuristic{})
		changed := 0
		for i := range rs {
			sr.seat = 0
			n := rs[i].HiddenCards(1, real[:])
			h := rs[i].HandSize(1)
			for k := uint64(0); k < 8; k++ {
				var w dirac.State
				w.CopyFrom(rs[i])
				sr.redeal(&w, k+1, k+77)
				if m := w.HiddenCards(1, got[:]); m != n || w.HandSize(1) != h {
					t.Fatalf("root %d: hidden %d/%d cards, hand %d/%d", i, m, n, w.HandSize(1), h)
				}
				same := true
				for j := 0; j < h; j++ {
					same = same && got[j] == real[j]
				}
				if cv && !same {
					t.Fatalf("root %d: clairvoyant world changed the opponent's hand", i)
				}
				if !same {
					changed++
				}
				var a, b [256]int
				for j := h; j < n; j++ {
					a[real[j]]++
					b[got[j]]++
				}
				if cv && a != b {
					t.Fatalf("root %d: clairvoyant library is not the real remaining cards", i)
				}
			}
		}
		if !cv && changed == 0 {
			t.Fatal("the ordinary deal never changed the opponent's hand")
		}
	}
}

// TestEarlyCutClairvoyantDefaultsOff: the four off-by-default knobs of the
// early-cut/clairvoyant round (EarlyCutGap, EarlyCutShare/Min, Clairvoyant,
// and the explore noise's unset alpha/eps) leave DefaultOptions and a
// default search exactly the pre-knob one: no cut, full Sims, and a Noise
// search with the alpha/eps fields unset is bit-identical to one with the
// azmcts defaults spelled out (explore.go's 0 = 0.3 / 0.25).
func TestEarlyCutClairvoyantDefaultsOff(t *testing.T) {
	if d := DefaultOptions(); d.EarlyCutGap || d.EarlyCutShare != 0 || d.EarlyCutMin != 0 || d.Clairvoyant || d.DirichletAlpha != 0 || d.DirichletEps != 0 {
		t.Fatalf("a knob is on by default: %+v", d)
	}
	o := DefaultOptions()
	o.Sims = 100
	plain := NewSearcher(o, Uniform{}, Heuristic{})
	rs := roots(30)
	for i := range rs {
		a := *plain.Search(rs[i], uint64(i))
		if a.Stats.EarlyCut != 0 || a.Stats.Simulations != o.Sims {
			t.Fatalf("root %d: the default search cut (%d sims)", i, a.Stats.Simulations)
		}
	}
	n := DefaultOptions()
	n.Sims, n.Noise = 60, true
	unset := NewSearcher(n, Uniform{}, Heuristic{})
	spelled := n
	spelled.DirichletAlpha, spelled.DirichletEps = 0.3, 0.25
	set := NewSearcher(spelled, Uniform{}, Heuristic{})
	var s dirac.State
	s.Init(v.Table, v.Decks[0], v.Decks[1], 7, dirac.Config{})
	checked := 0
	var d dirac.Decision
	for i := 0; i < 300 && checked < 10; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if d.Player == 0 {
			a := *unset.Search(&s, uint64(i))
			b := *set.Search(&s, uint64(i))
			if a.Visits != b.Visits || a.Choice != b.Choice || a.Prior != b.Prior {
				t.Fatalf("decision %d: the unset alpha/eps search differs from the spelled-out 0.3/0.25 one", i)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no searched decision")
	}
}
