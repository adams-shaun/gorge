package search

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for the remaining instant-timing keys (instkeys.go): HoldEps, CastPrior,
// DeferQ, Replan, RootSH, Reuse, InfoSeed and ReactAim, each shown acting
// when on and inert when off.

// rootSearcher is a searcher positioned at root s (seat 0's decision) for
// calling a final-move rule directly.
func rootSearcher(s *dirac.State, o Options) *Searcher {
	o.Sims = max(o.Sims, 4)
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	sr.root.CopyFrom(s)
	sr.root.StepInto(&sr.rootD)
	sr.seat, sr.seed = sr.rootD.Player, 0x1234
	return sr
}

// TestHoldEps: at a re-offered root cast (Burst Lightning in the own main
// 1) HoldEps 1 plays pass and leaves Waited (the record) alone; where the
// cast is not offered again (the opponent's end step, the own main 2's
// sorcery-speed creature) it never acts; HoldEps 0.5 acts on about half
// the decision seeds. In a full search the visits are those of the search
// without it.
func TestHoldEps(t *testing.T) {
	for _, c := range []struct {
		sc   waitScene
		want bool
	}{{waitScenes[3], true}, {waitScenes[8], false}, {waitScenes[1], false}, {waitScenes[2], true}} {
		s, a, pass := c.sc.root(t)
		o := DefaultOptions()
		o.HoldEps = 1
		sr := rootSearcher(s, o)
		r := tieRes(a, pass, 30, 10, 0.6, 0.4)
		got := sr.holdEps(r, 0)
		if (got == 1) != c.want || (sr.St.HoldEps == 1) != c.want || r.Waited != 0 {
			t.Errorf("%s: move %d HoldEps %d Waited %d, want pass %v", c.sc.name, got, sr.St.HoldEps, r.Waited, c.want)
		}
	}
	s, a, pass := waitScenes[3].root(t)
	o := DefaultOptions()
	o.HoldEps = 0.5
	sr := rootSearcher(s, o)
	n := 0
	for seed := uint64(0); seed < 400; seed++ {
		sr.seed = splitmix(seed)
		if sr.holdEps(tieRes(a, pass, 30, 10, 0.6, 0.4), 0) == 1 {
			n++
		}
	}
	if n < 160 || n > 240 {
		t.Errorf("HoldEps 0.5 passed %d of 400", n)
	}
	// a full search: the same visits, the move pass or the search's own
	o = timingOpts()
	off := *NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 7)
	o.HoldEps = 1
	on := *NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 7)
	if off.Stats.Searched != 1 {
		t.Fatal("the root was not searched")
	}
	if off.Visits != on.Visits || off.Q != on.Q {
		t.Fatal("HoldEps changed the search's statistics")
	}
	if off.Stats.HoldEpsEligible != 0 || off.Stats.HoldEps != 0 {
		t.Fatal("HoldEps counters set with it off")
	}
	if on.Stats.HoldEps == 1 && !(on.CandOne[on.Choice] && on.CandAct[on.Choice].Kind() == dirac.AkPass) {
		t.Fatal("HoldEps played something other than pass")
	}
	if on.Stats.HoldEps == 0 && on.Choice != off.Choice {
		t.Fatal("HoldEps changed the move without acting")
	}
}

// TestCastPrior: at seat 0's own main 1 a re-offered cast's prior is the
// unscaled one times B relative to pass (renormalised); at the opponent's
// end step (Burst Lightning's last window) nothing is scaled.
func TestCastPrior(t *testing.T) {
	for _, c := range []struct {
		sc    waitScene
		scale bool
	}{{waitScenes[3], true}, {waitScenes[8], false}} {
		s, _, _ := c.sc.root(t)
		var pts [2]point
		for i, b := range []float64{0, 0.25} {
			o := timingOpts()
			o.CastPrior = b
			sr := NewSearcher(o, cardPrior{}, passBot{})
			sr.root.CopyFrom(s)
			sr.root.StepInto(&sr.rootD)
			sr.seat = 0
			if !sr.buildPoint(&sr.root, &sr.rootD, passBot{}.Pick(&sr.root, &sr.rootD), &pts[i]) {
				t.Fatalf("%s: no point", c.sc.name)
			}
			if (b > 0 && c.scale) != (sr.St.CastPriorPoints == 1) {
				t.Fatalf("%s: CastPriorPoints %d", c.sc.name, sr.St.CastPriorPoints)
			}
		}
		pass := -1
		for i := 0; i < pts[0].n; i++ {
			if dirac.Action(pts[0].acts[pts[0].off[i]]).Kind() == dirac.AkPass {
				pass = i
			}
		}
		for i := 0; i < pts[0].n; i++ {
			a := dirac.Action(pts[0].acts[pts[0].off[i]])
			if !castOrActivate(a) {
				continue
			}
			want := pts[0].prior[i] / pts[0].prior[pass]
			if c.scale {
				want *= 0.25
			}
			if got := pts[1].prior[i] / pts[1].prior[pass]; math.Abs(got-want) > 1e-9*want {
				t.Errorf("%s: candidate %d ratio %g, want %g", c.sc.name, i, got, want)
			}
		}
	}
}

// TestDeferQ: the move A (key 1) at a re-offered root defers to pass when
// the seat's next node on pass's line (passTree: its key-1 child Q 0.7, 4
// visits) values it within EPS; not over EPS, not under DeferQMin visits,
// and never where A is not offered again.
func TestDeferQ(t *testing.T) {
	for _, c := range []struct {
		sc     waitScene
		qa     float64
		eps    float64
		minN   int
		wait   bool
		reason string
	}{
		{waitScenes[3], 0.72, 0.03, 0, true, "within EPS"},
		{waitScenes[3], 0.75, 0.03, 0, false, "over EPS"},
		{waitScenes[3], 0.72, 0.03, 5, false, "under DeferQMin"},
		{waitScenes[3], 0.72, 0.03, 4, true, "at DeferQMin"},
		{waitScenes[8], 0.72, 0.03, 0, false, "the last window"},
	} {
		s, a, pass := c.sc.root(t)
		o := DefaultOptions()
		o.DeferQ, o.DeferQMin = c.eps, c.minN
		sr := rootSearcher(s, o)
		passTree(sr)
		r := tieRes(a, pass, 30, 20, c.qa, 0.5)
		r.Keys[0], r.Keys[1] = 1, 2
		got := sr.finalMove(r, &sr.rootD, 0)
		if (got == 1) != c.wait || (r.Waited == 1) != c.wait || (sr.St.DeferQ == 1) != c.wait {
			t.Errorf("%s: move %d Waited %d DeferQ %d, want wait %v", c.reason, got, r.Waited, sr.St.DeferQ, c.wait)
		}
	}
}

// TestReplan: after seat 0 passes into an empty stack, the same position
// (same turn, view and options) is passed unsearched; a position whose
// option is not offered again (the own main 2's creature; Burst Lightning
// at the opponent's end step under the turn horizon) is searched; a record
// from another turn or of a cast is not used; off, nothing is skipped.
func TestReplan(t *testing.T) {
	for _, c := range []struct {
		sc   waitScene
		skip bool
	}{{waitScenes[3], true}, {waitScenes[2], true}, {waitScenes[1], false}, {waitScenes[8], false}} {
		s, a, pass := c.sc.root(t)
		var d dirac.Decision
		s.StepInto(&d)
		o := timingOpts()
		o.Replan = true
		sr := NewSearcher(o, HeuristicLeaf{}, passBot{})
		var st SeatState
		replanNote(s, &d, &Result{NActs: 1, Acts: [MaxPerm + 1]dirac.Action{pass}}, &st)
		res := sr.SeatSearch(s, &d, 5, &st)
		if (res.Stats.Replanned == 1) != c.skip || c.skip && (res.Stats.Searched != 0 || res.Acts[0] != pass || res.Record) {
			t.Errorf("%s: Replanned %d Searched %d act %v, want skip %v", c.sc.name, res.Stats.Replanned, res.Stats.Searched, res.Acts[0], c.skip)
		}
		if !c.skip {
			continue
		}
		// still passing: the record stands; a cast clears it
		if res = sr.SeatSearch(s, &d, 5, &st); res.Stats.Replanned != 1 {
			t.Errorf("%s: the second window was searched", c.sc.name)
		}
		replanNote(s, &d, &Result{NActs: 1, Acts: [MaxPerm + 1]dirac.Action{a}}, &st)
		if res = sr.SeatSearch(s, &d, 5, &st); res.Stats.Replanned != 0 {
			t.Errorf("%s: skipped after a cast", c.sc.name)
		}
		replanNote(s, &d, &Result{NActs: 1, Acts: [MaxPerm + 1]dirac.Action{pass}}, &st)
		st.rp.turn++
		if res = sr.SeatSearch(s, &d, 5, &st); res.Stats.Replanned != 0 {
			t.Errorf("%s: skipped on a record of another turn", c.sc.name)
		}
		st.rp.turn--
		st.rp.sig++
		if res = sr.SeatSearch(s, &d, 5, &st); res.Stats.Replanned != 0 {
			t.Errorf("%s: skipped with the view changed", c.sc.name)
		}
		// off: the same record is never read
		o.Replan = false
		off := NewSearcher(o, HeuristicLeaf{}, passBot{})
		replanNote(s, &d, &Result{NActs: 1, Acts: [MaxPerm + 1]dirac.Action{pass}}, &st)
		if res = off.SeatSearch(s, &d, 5, &st); res.Stats.Replanned != 0 || res.Stats.Searched != 1 {
			t.Errorf("%s: Replan off skipped", c.sc.name)
		}
	}
	set := cardsettest.Set(t, "fdn")
	// red-mirror games with it on (seat 0) play to the end and skip some
	// windows, every skip a pass
	tbl, da, db, err := set.Matchup(waitDeck, waitDeck)
	if err != nil {
		t.Fatal(err)
	}
	o := timingOpts()
	o.Sims, o.Replan = 16, true
	sr := NewSearcher(o, HeuristicLeaf{}, devBot{})
	n := 0
	for seed := uint64(1); seed <= 3; seed++ {
		var s dirac.State
		s.Init(tbl, da, db, seed, dirac.Config{})
		var st SeatState
		var d dirac.Decision
		for i := 0; i < 4000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if d.Player != 0 {
				s.Apply(d.Options[develop(&s, &d)])
				continue
			}
			r := sr.SeatSearch(&s, &d, seed, &st)
			if r.Stats.Replanned == 1 {
				n++
				if r.Acts[0].Kind() != dirac.AkPass {
					t.Fatal("a replanned window played no pass")
				}
			}
			r.Play(&s)
		}
	}
	t.Logf("%d windows replanned", n)
	if n == 0 {
		t.Error("no window replanned in three games")
	}
}

// TestRootSH: at a castable priority root sequential halving visits every
// candidate at least its first phase's share, the move is the best-Q
// survivor, and Stats.RootSH says so; off, PUCT's visits stand.
func TestRootSH(t *testing.T) {
	s, _, _ := waitScenes[3].root(t)
	o := timingOpts()
	o.Sims = 96
	off := *NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 9)
	o.RootSH = true
	sr := NewSearcher(o, HeuristicLeaf{}, passBot{})
	on := *sr.Search(s, 9)
	if off.Stats.RootSH != 0 || on.Stats.RootSH != 1 {
		t.Fatalf("RootSH counters off %d on %d", off.Stats.RootSH, on.Stats.RootSH)
	}
	m := on.N
	phases := max(1, int(math.Ceil(math.Log2(float64(m)))))
	per := int32(max(1, o.Sims/(phases*m)))
	for i := 0; i < m; i++ {
		if on.Visits[i] < per {
			t.Errorf("candidate %d: %d visits, under the first phase's %d", i, on.Visits[i], per)
		}
	}
	x := &sr.ik
	if x.shK != 1 && x.shK != 2 {
		t.Errorf("%d survivors at the end", x.shK)
	}
	for i := 0; i < x.shK; i++ {
		if c := x.shSurv[i]; on.Q[c] > on.Q[on.Choice] {
			t.Errorf("survivor %d's Q %g over the move's %g", c, on.Q[c], on.Q[on.Choice])
		}
	}
	if off.Visits == on.Visits {
		t.Error("RootSH left PUCT's visits")
	}
	// a root with no cast (the opponent's end step has one: use a root
	// whose candidates are pass and an activation only) is PUCT's
	s2, _, _ := waitScenes[10].root(t)
	var d dirac.Decision
	s2.StepInto(&d)
	cast := offers(s2, &d, func(a dirac.Action) bool { return a.Kind() == dirac.AkCast })
	r2 := sr.Search(s2, 9)
	if r2.Stats.Searched == 1 && !cast && r2.Stats.RootSH != 0 {
		t.Error("RootSH at a root with no cast")
	}
}

// TestReuse: a kept pass line two decisions back in the same turn warms
// the matching root candidates with F times its visits (Q kept); another
// sequence number or turn is not used; reuseKeep stores the seat's node
// under pass through the opponent's pass.
func TestReuse(t *testing.T) {
	s, _, _ := waitScenes[3].root(t)
	var d dirac.Decision
	s.StepInto(&d)
	o := timingOpts()
	o.Sims = 32
	base := *NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 4)
	if base.Stats.Searched != 1 {
		t.Fatal("the root was not searched")
	}
	keep := func(seq uint32, turn int32) SeatState {
		var st SeatState
		rs := &st.reuse
		rs.on, rs.turn, rs.seq, rs.n = true, turn, seq, 2
		rs.keys[0], rs.vis[0], rs.w[0] = base.Keys[0], 10, 7
		rs.keys[1], rs.vis[1], rs.w[1] = base.Keys[1], 4, 1
		return st
	}
	o.Reuse = 0.5
	for _, c := range []struct {
		seq  uint32
		turn int32
		warm bool
	}{{s.SeqNo() - 2, int32(s.Turn()), true}, {s.SeqNo() - 1, int32(s.Turn()), false}, {s.SeqNo() - 2, int32(s.Turn()) - 1, false}} {
		st := keep(c.seq, c.turn)
		sr := NewSearcher(o, HeuristicLeaf{}, passBot{})
		sr.ik.seatSt = &st
		r := *sr.Search(s, 4)
		var sum, sumB int32
		for i := 0; i < r.N; i++ {
			sum += r.Visits[i]
			sumB += base.Visits[i]
		}
		if c.warm {
			if r.Stats.Reused != 1 || r.Stats.ReusedVisits != 7 || sum != int32(r.Stats.Completed)+7 {
				t.Errorf("warm: Reused %d visits %d, root visits %d against %d", r.Stats.Reused, r.Stats.ReusedVisits, sum, sumB)
			}
		} else if r.Stats.Reused != 0 || r.Visits != base.Visits {
			t.Errorf("seq %d turn %d: warmed", c.seq, c.turn)
		}
	}
	// reuseKeep: root (A, pass) -> pass -> opponent node (its pass edge) ->
	// seat node with two visited edges and one unvisited
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	tr := &sr.t
	tr.reset()
	var pt point
	pt.n = 2
	pt.keys[0], pt.keys[1] = 11, 12
	root := tr.newNode(&pt)
	pt.opp = true
	passKey := Key(mix(mix(uint64(kindPriority), sr.actKey(s, dirac.MakeAction(dirac.AkPass, 0, 0, 0), true)), 0x6f70703a))
	pt.keys[0], pt.keys[1] = 99, passKey
	opp := tr.newNode(&pt)
	pt.opp, pt.n = false, 3
	pt.keys[0], pt.keys[1], pt.keys[2] = 21, 22, 23
	seat := tr.newNode(&pt)
	pe := tr.edges[tr.nodes[root].first].sib
	tr.edges[pe].next = opp
	oe := tr.edges[tr.nodes[opp].first].sib
	tr.edges[oe].next = seat
	e := tr.nodes[seat].first
	tr.edges[e].n, tr.edges[e].w = 6, 3
	e = tr.edges[e].sib
	tr.edges[e].n, tr.edges[e].w = 2, 2
	res := &Result{N: 2, Choice: 1, Stats: Stats{Searched: 1}}
	res.Keys[0], res.Keys[1] = 11, 12
	res.CandOne[0], res.CandOne[1], res.CandAct[1] = true, true, dirac.MakeAction(dirac.AkPass, 0, 0, 0)
	var st SeatState
	sr.reuseKeep(s, &d, res, &st)
	rs := &st.reuse
	if !rs.on || rs.n != 2 || rs.keys[0] != 21 || rs.vis[0] != 6 || rs.keys[1] != 22 || rs.w[1] != 2 || rs.seq != s.SeqNo() {
		t.Fatalf("kept %+v", rs)
	}
	res.Choice = 0 // a cast: nothing kept
	sr.reuseKeep(s, &d, res, &st)
	if st.reuse.on {
		t.Fatal("a cast's line kept")
	}
}

// TestInfoSeed: off, the worlds' seed is the decision seed; on, it is a
// function of the seat's information, so two decisions of one game that
// differ only in their sequence number (seat 0 and then seat 1 passed in
// between, nothing drawn or seen) deal the same worlds, while a different
// seat seed does not.
func TestInfoSeed(t *testing.T) {
	s, _, _ := waitScenes[3].root(t)
	o := timingOpts()
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	sr.Search(s, 3)
	if sr.ik.wseed != sr.seed {
		t.Fatal("InfoSeed off: the worlds' seed is not the decision seed")
	}
	o.InfoSeed = true
	on := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	on.Search(s, 3)
	w0 := on.ik.wseed
	// the next decision of seat 0 after passes only, in the same turn
	var s2 dirac.State
	s2.CopyFrom(s)
	var d dirac.Decision
	found := false
	for i := 0; i < 8; i++ {
		s2.StepInto(&d)
		if d.Kind != dirac.DkPriority || int(s2.Turn()) != s.Turn() {
			break
		}
		if i > 0 && d.Player == 0 {
			found = true
			break
		}
		s2.Apply(d.Options[optOf(&d, isPassAct)])
	}
	if !found {
		t.Skip("no later seat-0 priority decision in the turn")
	}
	same := s2.HandSize(0) == s.HandSize(0) && s2.HandSize(1) == s.HandSize(1) && s2.LibrarySize(0) == s.LibrarySize(0) &&
		s2.LibrarySize(1) == s.LibrarySize(1) && s2.SeenCards(1, 0, make([]dirac.CardID, 128)) == s.SeenCards(1, 0, make([]dirac.CardID, 128))
	on.Search(&s2, 3)
	if s2.SeqNo() == s.SeqNo() {
		t.Fatal("the same decision")
	}
	if same && on.ik.wseed != w0 {
		t.Errorf("same information, different worlds' seeds")
	}
	on.Search(s, 4)
	if on.ik.wseed == w0 {
		t.Error("another seat seed, the same worlds' seed")
	}
	sr.Search(&s2, 3)
	if sr.ik.wseed == w0 {
		t.Error("off: the worlds' seed followed the information")
	}
}

// aimRoot is seat 0's own precombat main with an empty stack, Burst
// Lightning castable and an opponent's creature on the battlefield.
func aimRoot(t *testing.T) *dirac.State {
	return deckRoot(t, waitDeck, develop, func(s *dirac.State, d *dirac.Decision) bool {
		if !noLand(at(burst, false, 0, kop.StepMain1, false))(s, d) {
			return false
		}
		for i := 0; i < s.NumObjs(); i++ {
			o := dirac.Obj(i)
			if s.ObjZone(o) == kop.ZoneBattlefield && s.Controller(o) == 1 && s.IsCreature(o) {
				return true
			}
		}
		return false
	})
}

// TestReactAim: under the horizon react a quiet own-main-1 Burst Lightning is
// held (no root candidate); with ReactAim the probe finds the opponent's
// creature among its targets and keeps it. Once it is cast, the target
// decision's candidates are the opponent's objects only (no player, no
// own object) and the bot's answer is one; off, the players stay.
func TestReactAim(t *testing.T) {
	s := aimRoot(t)
	isBurst := func(a dirac.Action) bool { return castsCard(s, a, burst, false) }
	o := timingOpts()
	o.InstWin, o.ReactHorizon, o.ReactRoot = InstWinReact, true, true
	plain := NewSearcher(o, HeuristicLeaf{}, passBot{})
	rp := plain.Search(s, 2)
	if rp.Stats.Searched == 1 && rootHas(plain, rp, isBurst) {
		t.Fatal("react kept the quiet pre-combat Burst Lightning")
	}
	o.ReactAim = true
	aim := NewSearcher(o, HeuristicLeaf{}, passBot{})
	ra := aim.Search(s, 2)
	if ra.Stats.Searched != 1 || !rootHas(aim, ra, isBurst) || ra.Stats.ReactAimKept == 0 {
		t.Fatalf("reactaim: searched %d, Burst a candidate %v, kept %d", ra.Stats.Searched, rootHas(aim, ra, isBurst), ra.Stats.ReactAimKept)
	}
	// cast it: the target decision
	var d dirac.Decision
	s.StepInto(&d)
	var tg dirac.State
	tg.CopyFrom(s)
	tg.Apply(d.Options[optOf(&d, isBurst)])
	tg.StepInto(&d)
	if d.Kind != dirac.DkTarget || d.Player != 0 {
		t.Fatalf("no target decision after the cast: %v", d.Kind)
	}
	players := offers(&tg, &d, func(a dirac.Action) bool { return a.Target() >= dirac.PlayerTarget0 })
	if !players {
		t.Fatal("Burst Lightning offered no player target")
	}
	oppObj := func(a dirac.Action) bool {
		return a.Kind() == dirac.AkTarget && a.Target() < dirac.PlayerTarget0 && tg.Controller(dirac.Obj(a.Target())) == 1
	}
	r := aim.Search(&tg, 2)
	if r.Stats.Searched == 1 {
		for c := 0; c < r.N; c++ {
			if !oppObj(dirac.Action(aim.RootCand(c)[0])) {
				t.Fatalf("candidate %d is no opponent's object", c)
			}
		}
	} else if !oppObj(r.Acts[0]) {
		t.Fatalf("unsearched answer %v is no opponent's object", r.Acts[0])
	}
	if b := aim.instBot(&tg, &d, optOf(&d, func(a dirac.Action) bool { return a.Target() >= dirac.PlayerTarget0 })); !oppObj(d.Options[b]) {
		t.Fatal("the bot's player answer was kept")
	}
	o.ReactAim = false
	off := NewSearcher(o, HeuristicLeaf{}, passBot{})
	r = off.Search(&tg, 2)
	if r.Stats.Searched != 1 || !rootHas(off, r, func(a dirac.Action) bool { return a.Target() >= dirac.PlayerTarget0 }) {
		t.Fatal("off: no player target among the candidates")
	}
}

// TestInstKeysValidate: the keys' bad values and combinations are refused, and
// the untap-step horizon is accepted with any reader of it (ReadsHorizon).
func TestInstKeysValidate(t *testing.T) {
	ok := func(f func(o *Options)) Options {
		o := DefaultOptions()
		f(&o)
		return o
	}
	for _, o := range []Options{
		ok(func(o *Options) { o.HoldEps = 1.5 }),
		ok(func(o *Options) { o.CastPrior = -1 }),
		ok(func(o *Options) { o.CastPrior = math.Inf(1) }),
		ok(func(o *Options) { o.DeferQ = 2 }),
		ok(func(o *Options) { o.DeferQMin = 3 }),
		ok(func(o *Options) { o.DeferQ, o.CachedWorlds = 0.02, 2 }),
		ok(func(o *Options) { o.Reuse = -1 }),
		ok(func(o *Options) { o.Reuse, o.Batch = 0.5, 4 }),
		ok(func(o *Options) { o.RootSH, o.CachedWorlds = true, 2 }),
		ok(func(o *Options) { o.RootSH, o.ForcedK = true, 2 }),
		ok(func(o *Options) { o.ReactAim = true }),
		ok(func(o *Options) { o.ReactAim, o.InstWin = true, InstWinReact }),
		ok(func(o *Options) { o.WaitTieUntap = true }),
	} {
		if o.Validate() == nil {
			t.Errorf("accepted %+v", o)
		}
	}
	for _, o := range []Options{
		ok(func(o *Options) { o.HoldEps, o.WaitTieUntap = 0.15, true }),
		ok(func(o *Options) { o.CastPrior, o.WaitTieUntap = 0.1, true }),
		ok(func(o *Options) { o.DeferQ, o.DeferQMin, o.WaitTieUntap = 0.03, 4, true }),
		ok(func(o *Options) { o.Replan, o.WaitTieUntap = true, true }),
		ok(func(o *Options) { o.RootSH, o.Reuse, o.InfoSeed = true, 0.5, true }),
		ok(func(o *Options) { o.ReactAim, o.InstWin, o.ReactHorizon = true, InstWinReact, true }),
	} {
		if err := o.Validate(); err != nil {
			t.Errorf("refused: %v", err)
		}
	}
}

// TestInstKeysZeroAllocs: a search with every one of these keys on allocates
// nothing (the SeatSearch path included).
func TestInstKeysZeroAllocs(t *testing.T) {
	o := timingOpts()
	o.Sims = 32
	o.HoldEps, o.CastPrior, o.DeferQ, o.Replan, o.RootSH, o.Reuse, o.InfoSeed, o.WaitTieUntap = 0.5, 0.3, 0.03, true, true, 0.5, true, true
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	var st SeatState
	for _, sc := range []waitScene{waitScenes[3], waitScenes[2]} {
		s, _, _ := sc.root(t)
		var d dirac.Decision
		s.StepInto(&d)
		if allocs := testing.AllocsPerRun(5, func() { sr.SeatSearch(s, &d, 1, &st) }); allocs != 0 {
			t.Fatalf("%s: allocs per search = %v", sc.name, allocs)
		}
	}
	o = timingOpts()
	o.Sims, o.InstWin, o.ReactHorizon, o.ReactAim = 32, InstWinReact, true, true
	sr = NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	s := aimRoot(t)
	if allocs := testing.AllocsPerRun(5, func() { sr.Search(s, 1) }); allocs != 0 {
		t.Fatalf("reactaim: allocs per search = %v", allocs)
	}
}

// devBot is the scenario driver as a search bot: a land, a creature in the
// active player's main phase, else pass.
type devBot struct{}

func (devBot) Pick(s *dirac.State, d *dirac.Decision) int { return develop(s, d) }
