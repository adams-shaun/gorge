package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for the timing options WaitSorc and PassMax, the :react scope
// (ReactRoot, ReactDan, ReactTarget) and :defer.

// TestWaitSorc: under WaitSorcOn, a sorcery-speed move waits by WaitSorc
// and an instant-speed one by WaitTie: the Raider in main 1 at margins
// (WaitTie 1, WaitSorc 0) never waits and at (0, 1) does; Burst Lightning
// in main 1 at (1, 0) waits and at (0, 1) does not. Without WaitSorcOn
// WaitTie is both margins (the original rule).
func TestWaitSorc(t *testing.T) {
	sorc, inst := waitScenes[0], waitScenes[3]
	for _, c := range []struct {
		sc         waitScene
		tie, sorcM float64
		on, want   bool
	}{
		{sorc, 1, 0, true, false},
		{sorc, 0, 1, true, true},
		{sorc, 1, 0, false, true},
		{inst, 1, 0, true, true},
		{inst, 0, 1, true, false},
		{inst, 1, 0, false, true},
	} {
		s, a, pass := c.sc.root(t)
		o := DefaultOptions()
		o.WaitTie, o.WaitSorc, o.WaitSorcOn = c.tie, c.sorcM, c.on
		got, st := ruleAt(s, o, tieRes(a, pass, 10, 10, 0.5, 0.5), 0)
		if (got == 1) != c.want || (st.WaitTie == 1) != c.want {
			t.Errorf("%s, waittie %g waitsorc %g (on %v): move %d WaitTie %d, want wait %v", c.sc.name, c.tie, c.sorcM, c.on, got, st.WaitTie, c.want)
		}
	}
	o := DefaultOptions()
	o.WaitSorc = 0.02
	if o.Validate() == nil {
		t.Error("WaitSorc without WaitSorcOn accepted")
	}
	o.WaitSorcOn, o.PassMax = true, true
	if err := o.Validate(); err != nil {
		t.Errorf("waitsorc + passmax refused: %v", err)
	}
	o = DefaultOptions()
	o.PassMax = true
	if o.Validate() == nil {
		t.Error("PassMax without a wait margin accepted")
	}
	o.WaitTie, o.CachedWorlds = 0.02, 2
	if o.Validate() == nil {
		t.Error("PassMax with CachedWorlds accepted")
	}
}

// passTree builds a root with candidates (A, pass) whose pass child is an
// opponent node with two replies (5 and 3 visits), the most-visited one
// leading to a seat node with children of Q 0.7 (4 visits), 0.9 (1 visit:
// under the floor max(2, 0.1 x 10)) and 0.6 (10 visits).
func passTree(sr *Searcher) {
	t := &sr.t
	t.reset()
	var pt point
	pt.n = 2
	pt.keys[0], pt.keys[1] = 1, 2
	root := t.newNode(&pt)
	pt.opp = true
	opp := t.newNode(&pt)
	pt.opp, pt.n = false, 3
	pt.keys[2] = 3
	seat := t.newNode(&pt)
	pass := t.nodes[root].first
	pass = t.edges[pass].sib
	t.edges[pass].next, t.edges[pass].n, t.edges[pass].w = opp, 20, 10
	r0 := t.nodes[opp].first
	r1 := t.edges[r0].sib
	t.edges[r0].n, t.edges[r0].next = 5, seat
	t.edges[r1].n = 3
	e := t.nodes[seat].first
	for _, c := range []struct {
		n int32
		q float64
	}{{4, 0.7}, {1, 0.9}, {10, 0.6}} {
		t.edges[e].n, t.edges[e].w = c.n, c.q*float64(c.n)
		e = t.edges[e].sib
	}
}

// TestPassMax: passLine follows pass through the opponent's most-visited
// reply to the seat's next node and takes its best child over the floor
// (0.7: the 0.9 child has one visit); the wait then compares Q(A) with that
// line, not pass's average: A at 0.65 against pass's average 0.5 waits
// only under PassMax (EPS 0.01).
func TestPassMax(t *testing.T) {
	s, a, pass := waitScenes[3].root(t)
	o := DefaultOptions()
	o.Sims, o.WaitTie = 4, 0.01
	for _, pm := range []bool{false, true} {
		o.PassMax = pm
		sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
		sr.root.CopyFrom(s)
		passTree(sr)
		if v, ok := sr.passLine(1); !ok || v != 0.7 {
			t.Fatalf("passLine %g %v, want 0.7", v, ok)
		}
		got := sr.waitTie(tieRes(a, pass, 20, 20, 0.65, 0.5), 0)
		if (got == 1) != pm || (sr.St.PassMaxUsed == 1) != pm {
			t.Errorf("passmax %v: move %d PassMaxUsed %d", pm, got, sr.St.PassMaxUsed)
		}
	}
	// a pass child with no line (unexpanded): the average stands
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	sr.root.CopyFrom(s)
	passTree(sr)
	sr.t.edges[sr.t.edges[sr.t.nodes[0].first].sib].next = -1
	if _, ok := sr.passLine(1); ok {
		t.Fatal("passLine through an unexpanded pass child")
	}
}

// TestReactDanWindows: under ReactDan the react rule holds an own
// instant-speed cast into an empty stack unless the opponent did something
// this turn or the untap-step horizon offers it no later window
// (laterStep); under ReactRoot it holds at the root's decision only.
func TestReactDanWindows(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	o := DefaultOptions()
	o.InstWin, o.ReactDan = InstWinReact, true
	sr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
	or := o
	or.ReactRoot = true
	rr := NewSearcher(or, &fakeNet{val: -1}, Heuristic{})
	var held, last, fired int
	for seed := uint64(1); seed <= 30; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x77)
		var tr reactTrack
		var d dirac.Decision
		for i := 0; i < 2500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			tr.observe(s, 0)
			if d.Kind == dirac.DkPriority && d.Player == 0 && s.StackLen() == 0 {
				sr.seat, sr.curReact = 0, &tr
				rr.seat, rr.curReact = 0, &tr
				for j := 0; j < int(d.N); j++ {
					a := d.Options[j]
					if a.Kind() != dirac.AkCast {
						continue
					}
					def := &s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))]
					if def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0 {
						continue
					}
					blocked := sr.instBlocked(s, a)
					later := laterStep(s.StepNow(), true, pastTurn(s.ActivePlayer(), s.NextTurnPlayer(), 0, s.OutlastsTurn(a)))
					switch {
					case !later:
						last++
						if blocked {
							t.Fatalf("seed %d: held at its last window (step %d, active %d)", seed, s.StepNow(), s.ActivePlayer())
						}
					case tr.firedNow(s, 0):
						fired++
						if blocked {
							t.Fatalf("seed %d: held after an opponent event", seed)
						}
					default:
						held++
						if !blocked {
							t.Fatalf("seed %d: not held with no event before its last window", seed)
						}
					}
					rr.rootDec = false
					if rr.instBlocked(s, a) {
						t.Fatalf("seed %d: ReactRoot held an in-walk cast", seed)
					}
					rr.rootDec = true
					if rr.instBlocked(s, a) != blocked {
						t.Fatalf("seed %d: ReactRoot at the root differs from the rule", seed)
					}
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	t.Logf("held %d, last window %d, after an event %d", held, last, fired)
	if held == 0 || last == 0 || fired == 0 {
		t.Fatal("a window case was never reached")
	}
}

// TestReactTarget: under ReactDan the root's held casts are listed for the
// record (RootHeld, HeldSingle) only with ReactTarget; the move is the
// same either way, and so are the statistics of every searched root (a
// held singleton is a one-candidate record of an unsearched root).
func TestReactTarget(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	mk := func(target bool) *Searcher {
		sr := reactSearcher()
		sr.Opts.ReactDan, sr.Opts.ReactTarget = true, target
		return sr
	}
	off, on := mk(false), mk(true)
	var withHeld int
	for seed := uint64(1); seed <= 6; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x21)
		var st0, st1 SeatState
		var d dirac.Decision
		for i := 0; i < 2500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			st0.ObserveReact(s, 0)
			st1.ObserveReact(s, 0)
			if d.Player != 0 || d.Kind != dirac.DkPriority {
				s.Apply(pol.Pick(&d))
				continue
			}
			r0 := *off.SeatSearch(s, &d, seed, &st0)
			r1 := *on.SeatSearch(s, &d, seed, &st1)
			if len(off.RootHeld()) != 0 || r0.HeldSingle {
				t.Fatalf("seed %d: held casts listed without ReactTarget", seed)
			}
			if len(on.RootHeld()) > 0 {
				withHeld++
			}
			if r0.Acts != r1.Acts || !r1.HeldSingle && (r0.Visits != r1.Visits || r0.Q != r1.Q) {
				t.Fatalf("seed %d: ReactTarget changed the search", seed)
			}
			r1.Play(s)
		}
	}
	if withHeld == 0 {
		t.Fatal("no root held a cast")
	}
}

// TestDefer: :defer=w holds the seat's own instant-speed casts in its own
// upkeep and draw (not in response to an opponent's object), and scales
// their main-1 prior by w; off, nothing is held.
func TestDefer(t *testing.T) {
	tbl, da, db := reactMatchup(t)
	o := DefaultOptions()
	o.Defer = 0.5
	sr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
	plain := NewSearcher(DefaultOptions(), &fakeNet{val: -1}, Heuristic{})
	var held, main1 int
	for seed := uint64(1); seed <= 20; seed++ {
		s := dirac.NewConfig(tbl, da, db, seed, dirac.Config{NoElide: true})
		pol := dirac.NewRandomPolicy(seed ^ 0x44)
		var d dirac.Decision
		for i := 0; i < 2500; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Kind == dirac.DkPriority && d.Player == 0 {
				sr.seat, plain.seat = 0, 0
				for j := 0; j < int(d.N); j++ {
					a := d.Options[j]
					want := sr.instSpeedCast(s, a) && (s.StepNow() == kop.StepUpkeep || s.StepNow() == kop.StepDraw)
					if got := sr.instBlocked(s, a); got != want {
						t.Fatalf("seed %d step %d: blocked %v, want %v", seed, s.StepNow(), got, want)
					}
					if plain.instBlocked(s, a) {
						t.Fatal("blocked with defer off")
					}
					if want {
						held++
					}
					if sr.instSpeedCast(s, a) && s.StepNow() == kop.StepMain1 {
						main1++
					}
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	if held == 0 || main1 == 0 {
		t.Fatalf("vacuous: held %d main1 %d", held, main1)
	}
	var pt point
	pt.n = 2
	pt.off[1], pt.off[2] = 1, 2
	pt.prior[0], pt.prior[1] = 0.5, 0.5
	sr.deferScale(&dirac.State{}, &pt) // not main 1: unchanged
	if pt.prior[0] != 0.5 {
		t.Fatal("deferScale outside main 1")
	}
}

// TestSkipPass: :skippass passes the walk's empty-stack priority windows
// outside a main phase (level 2: also the non-active seat's main-phase
// ones) without a point or the bot; never the root. Off, nothing is
// skipped and the search is the old one; on, the root is the same decision
// and its search runs.
func TestSkipPass(t *testing.T) {
	s, _, _ := waitScenes[0].root(t) // seat 0's own main 1, no land play
	o := timingOptsLimit()
	o.OpponentNodes = true
	off := *NewSearcher(o, Uniform{}, Heuristic{}).Search(s, 5)
	if off.Stats.SkipPassed != 0 {
		t.Fatal("skipped with SkipPass off")
	}
	for _, lvl := range []int{1, 2} {
		o.SkipPass = lvl
		sr := NewSearcher(o, Uniform{}, Heuristic{})
		on := *sr.Search(s, 5)
		if on.Stats.SkipPassed == 0 || on.Stats.Searched != 1 || on.N != off.N || on.Keys != off.Keys {
			t.Fatalf("level %d: skipped %d searched %d N %d/%d", lvl, on.Stats.SkipPassed, on.Stats.Searched, on.N, off.N)
		}
	}
	// the window rule, on random games' priority decisions
	tbl, da, db := reactMatchup(t)
	for _, lvl := range []int{1, 2} {
		o.SkipPass = lvl
		sr := NewSearcher(o, Uniform{}, Heuristic{})
		var skipped, kept int
		for seed := uint64(1); seed <= 4; seed++ {
			var w world
			w.s.Init(tbl, da, db, seed, dirac.Config{NoElide: true})
			pol := dirac.NewRandomPolicy(seed ^ 0x5)
			for i := 0; i < 1500; i++ {
				w.s.StepInto(&w.d)
				if w.d.Kind == dirac.DkNone {
					break
				}
				st := w.s.StepNow()
				main := st == kop.StepMain1 || st == kop.StepMain2
				want := w.d.Kind == dirac.DkPriority && w.s.StackLen() == 0 && (!main || lvl == 2 && w.d.Player != w.s.ActivePlayer())
				if got := sr.skipWindow(&w); got != want {
					t.Fatalf("level %d step %d player %d stack %d: skip %v, want %v", lvl, st, w.d.Player, w.s.StackLen(), got, want)
				} else if got {
					skipped++
					continue
				}
				kept++
				w.s.Apply(pol.Pick(&w.d))
			}
		}
		if skipped == 0 || kept == 0 {
			t.Fatalf("level %d vacuous: %d skipped %d kept", lvl, skipped, kept)
		}
	}
}
