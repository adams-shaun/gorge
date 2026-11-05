package search

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// infoRoots plays vanilla games (both seats random) and calls f at up to
// max seat-0 decisions that a default search searches.
func infoRoots(t *testing.T, max int, f func(s *dirac.State)) {
	t.Helper()
	probe := NewSearcher(DefaultOptions(), &fakeNet{val: -1}, Heuristic{})
	n := 0
	for g := uint64(1); g <= 40 && n < max; g++ {
		var s dirac.State
		s.Init(v.Table, v.Decks[g%5], v.Decks[(g+2)%5], g, dirac.Config{})
		pol := dirac.NewRandomPolicy(g ^ 0x77)
		var d dirac.Decision
		for i := 0; i < 3000 && n < max; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Player == 0 && i%3 == 0 {
				if r := probe.Search(&s, g); r.Stats.Searched > 0 {
					f(&s)
					n++
				}
			}
			s.Apply(pol.Pick(&d))
		}
	}
	if n == 0 {
		t.Fatal("no searched root")
	}
}

// Off is the search as without the keys, and a key on with a share so
// small that no simulation re-deals is too (the bookkeeping is inert).
func TestInfoKeysInertWhenOff(t *testing.T) {
	base := DefaultOptions()
	base.Sims = 60
	tiny := base
	tiny.OppView = 1e-300
	tinyFair := base
	tinyFair.OppFair = 1e-300
	zero := base
	zero.HoldBonus = 0
	a := NewSearcher(base, &fakeNet{val: -1}, Heuristic{})
	b := NewSearcher(tiny, &fakeNet{val: -1}, Heuristic{})
	z := NewSearcher(zero, &fakeNet{val: -1}, Heuristic{})
	infoRoots(t, 25, func(s *dirac.State) {
		ra := *a.Search(s, 5)
		rb := *b.Search(s, 5)
		rz := *z.Search(s, 5)
		if !sameSearch(&ra, &rb) || !sameSearch(&ra, &rz) || rb.Stats.OppViewSims != 0 {
			t.Fatalf("an off/inert key changed the search")
		}
	})
	// OppFair's real-hand simulations skip the opponent's points, so even a
	// share of 1e-300 changes the tree once the walk meets one; it must
	// still never re-deal
	c := NewSearcher(tinyFair, &fakeNet{val: -1}, Heuristic{})
	infoRoots(t, 5, func(s *dirac.State) {
		if r := c.Search(s, 5); r.Stats.OppViewSims != 0 {
			t.Fatal("oppfair 1e-300 re-dealt")
		}
	})
}

// rootVisits sums the root's edge visits (the tree just searched).
func rootVisits(sr *Searcher) (n int32) {
	t := &sr.t
	for e := t.nodes[0].first; e >= 0; e = t.edges[e].sib {
		n += t.edges[e].n
	}
	return n
}

// Under OppView and OppFair the root's visits are exactly the simulations
// that were not re-dealt (the root is the seat's point): every completed
// simulation but the re-dealt ones.
func TestOppViewOppFairRootFromRealHands(t *testing.T) {
	for _, f := range []func(o *Options){
		func(o *Options) { o.OppView = 0.5 },
		func(o *Options) { o.OppFair = 0.5 },
		func(o *Options) { o.OppView = 1 },
	} {
		o := DefaultOptions()
		o.Sims = 80
		f(&o)
		sr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
		dealt := 0
		infoRoots(t, 30, func(s *dirac.State) {
			r := sr.Search(s, 9)
			if got, want := int(rootVisits(sr)), r.Stats.Completed-sr.hidDealtCommits; got != want {
				t.Fatalf("oppview %g oppfair %g: root visits %d, want %d (completed %d, re-dealt backups %d)",
					o.OppView, o.OppFair, got, want, r.Stats.Completed, sr.hidDealtCommits)
			}
			dealt += sr.hidDealtCommits
		})
		if dealt == 0 {
			t.Fatalf("oppview %g oppfair %g: no re-dealt backup", o.OppView, o.OppFair)
		}
	}
}

// commitInfo's rule on a synthetic path seat -> opponent -> seat: a
// re-dealt simulation updates the opponent's entry only (node, edge and its
// availability marks), an OppFair real-hand one the seat's only, and a plain
// (or OppView real-hand) one every entry.
func TestCommitInfoSides(t *testing.T) {
	var pt point
	pt.n = 2
	pt.keys[0], pt.keys[1] = 1, 2
	for _, tc := range []struct {
		mode uint8
		want [3]bool
	}{{hidOpp, [3]bool{false, true, false}}, {hidSeat, [3]bool{true, false, true}}, {hidAll, [3]bool{true, true, true}}} {
		sr := NewSearcher(DefaultOptions(), Uniform{}, Heuristic{})
		sr.hidOn, sr.hidMode = true, tc.mode
		tr := &sr.t
		tr.reset()
		for i := 0; i < 3; i++ {
			pt.opp = i == 1
			n := tr.newNode(&pt)
			sr.pathN[i], sr.pathE[i], sr.pathOpp[i] = n, tr.nodes[n].first, pt.opp
			sr.markLo[i] = int32(2 * i)
			sr.marks[2*i], sr.marks[2*i+1] = tr.nodes[n].first, tr.edges[tr.nodes[n].first].sib
		}
		sr.commit(3, 6, 0.7)
		for i := 0; i < 3; i++ {
			nd := &tr.nodes[sr.pathN[i]]
			e := &tr.edges[sr.pathE[i]]
			got := nd.n == 1 && e.n == 1 && e.w == 0.7 && e.avail == 1 && tr.edges[e.sib].avail == 1
			none := nd.n == 0 && e.n == 0 && e.avail == 0 && tr.edges[e.sib].avail == 0
			if tc.want[i] && !got || !tc.want[i] && !none {
				t.Fatalf("mode %d entry %d: node n %d, edge n %d w %v avail %d/%d", tc.mode, i, nd.n, e.n, e.w, e.avail, tr.edges[e.sib].avail)
			}
		}
	}
}

// The re-deal keeps the seat's hidden multiset (hand and library) and its
// zone sizes, and changes the hand in some world.
func TestHidRedealKeepsPool(t *testing.T) {
	o := DefaultOptions()
	o.OppView = 1
	sr := NewSearcher(o, &fakeNet{val: -1}, Heuristic{})
	changed := 0
	infoRoots(t, 10, func(s *dirac.State) {
		sr.Search(s, 3) // sets seat and seed
		for sim := 0; sim < 4; sim++ {
			var w dirac.State
			w.CopyFrom(s)
			sr.hidBegin(sim)
			var before, after [2 * dirac.MaxDeck]dirac.CardID
			nb := w.HiddenCards(sr.seat, before[:])
			h0 := w.HandSize(sr.seat)
			sr.hidRedeal(&w)
			na := w.HiddenCards(sr.seat, after[:])
			if na != nb || w.HandSize(sr.seat) != h0 {
				t.Fatal("re-deal changed the zone sizes")
			}
			var cnt [4096]int
			for i := 0; i < nb; i++ {
				cnt[before[i]]++
				cnt[after[i]]--
			}
			for _, c := range cnt {
				if c != 0 {
					t.Fatal("re-deal changed the seat's hidden multiset")
				}
			}
			if !sameCards(before[:h0], after[:h0]) {
				changed++
			}
		}
	})
	if changed == 0 {
		t.Fatal("the re-deal never changed a hand")
	}
}

func sameCards(a, b []dirac.CardID) bool {
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// HoldBonus adds LAMBDA to a non-terminal leaf on the opponent's turn
// exactly when the seat holds an instant-speed card its untapped lands
// cover; never on its own turn.
func TestHoldBonusLeaf(t *testing.T) {
	o := DefaultOptions()
	o.HoldBonus = 0.05
	sr := NewSearcher(o, &fakeNet{val: 0.5}, Heuristic{})
	sr.seat = 0
	var on, off, own int
	for g := uint64(1); g <= 60; g++ {
		var w world
		w.s.Init(v.Table, v.Decks[g%5], v.Decks[(g+2)%5], g, dirac.Config{})
		pol := dirac.NewRandomPolicy(g)
		for i := 0; i < 2000; i++ {
			w.s.StepInto(&w.d)
			if w.d.Kind == dirac.DkNone {
				break
			}
			val, term := sr.leaf(&w)
			if !term {
				want := 0.5
				if w.s.ActivePlayer() != 0 && holdsCoverable(&w.s, 0) {
					want += 0.05
					on++
				} else if w.s.ActivePlayer() != 0 {
					off++
				} else {
					own++
				}
				if val != want {
					t.Fatalf("game %d step %d: leaf %v, want %v", g, i, val, want)
				}
			}
			w.s.Apply(pol.Pick(&w.d))
		}
	}
	if on == 0 || off == 0 || own == 0 {
		t.Fatalf("cases not covered: bonus %d, opponent's turn without %d, own turn %d", on, off, own)
	}
}

// holdsCoverable is the test's own reading of the rule.
func holdsCoverable(s *dirac.State, p uint8) bool {
	t := s.Table()
	untapped := 0
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) == kop.ZoneBattlefield && s.Controller(ob) == p && !s.Tapped(ob) && t.Cards[s.ObjCard(ob)].Types&kop.TLand != 0 {
			untapped++
		}
	}
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) != kop.ZoneHand || s.ObjOwner(ob) != p {
			continue
		}
		c := &t.Cards[s.ObjCard(ob)]
		if c.Types&kop.TLand == 0 && (c.Types&kop.TInstant != 0 || c.Keywords&kop.KwFlash != 0) && manaValue(&c.Cost) <= untapped {
			return true
		}
	}
	return false
}

func TestValidateInfo(t *testing.T) {
	bad := []func(o *Options){
		func(o *Options) { o.HoldBonus = -0.1 },
		func(o *Options) { o.OppView = 1.5 },
		func(o *Options) { o.OppFair = -1 },
		func(o *Options) { o.OppView, o.OppFair = 0.5, 0.5 },
		func(o *Options) { o.OppView, o.Batch = 0.5, 2 },
		func(o *Options) { o.OppFair, o.NodeCache = 0.5, 64 },
	}
	for i, f := range bad {
		o := DefaultOptions()
		f(&o)
		if o.Validate() == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	o := DefaultOptions()
	o.HoldBonus, o.OppView = 0.05, 0.8
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
}
