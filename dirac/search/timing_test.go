package search

import (
	"math"
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for the instant-timing options: Options.LandSearch
// (points.go priorityCands), HoldUpkeep (holdupkeep.go) and CardSims /
// CardSimsForce (cardsims.go). The scenarios are FDN games of a red deck --
// Mountains, Burst Lightning (an instant, kicker {4}) and Fanatical
// Firebrand (a {R} haste 1/1 with "{T}, sacrifice: 1 damage to any
// target", an instant-speed activation) -- played by a script to the
// decision a test needs.

const (
	burst     = "Burst Lightning"
	firebrand = "Fanatical Firebrand"
)

var redDeck = []cardset.Entry{{Name: "Mountain", Count: 17}, {Name: burst, Count: 12}, {Name: firebrand, Count: 11}}

// cardOf is the table name of the card option a acts with (its source).
func cardOf(s *dirac.State, a dirac.Action) string {
	return s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))].Name
}

// castsCard reports whether a casts (or, act, activates) the named card.
func castsCard(s *dirac.State, a dirac.Action, name string, act bool) bool {
	k := a.Kind()
	return (k == dirac.AkCast || act && k == dirac.AkActivate) && cardOf(s, a) == name
}

func offers(s *dirac.State, d *dirac.Decision, f func(dirac.Action) bool) bool {
	for i := 0; i < int(d.N); i++ {
		if f(d.Options[i]) {
			return true
		}
	}
	return false
}

// scriptPick is the scenario driver: a land when one is offered, else
// (develop) a Firebrand in the active player's main phase, else pass, Done
// or the first option. It never casts Burst Lightning, so the cards pile
// up in hand with the lands to cast them.
func scriptPick(s *dirac.State, d *dirac.Decision, develop bool) int {
	switch d.Kind {
	case dirac.DkPriority:
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == dirac.AkPlayLand {
				return i
			}
		}
		if develop && d.Player == s.ActivePlayer() {
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; castsCard(s, a, firebrand, false) {
					return i
				}
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == dirac.AkPass {
				return i
			}
		}
	case dirac.DkAttackers, dirac.DkBlockers:
		if i := doneIndex(d); i >= 0 {
			return i
		}
	}
	return 0
}

// redRoot plays the red mirror from seeds 1.. with pick until want holds
// at a decision and returns that position (stepped into the decision).
func redRoot(t *testing.T, pick func(s *dirac.State, d *dirac.Decision) int, want func(s *dirac.State, d *dirac.Decision) bool) *dirac.State {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	tbl, a, b, err := set.Matchup(redDeck, redDeck)
	if err != nil {
		t.Fatal(err)
	}
	s := new(dirac.State)
	var d dirac.Decision
	for seed := uint64(1); seed <= 40; seed++ {
		s.Init(tbl, a, b, seed, dirac.Config{})
		for n := 0; n < 4000; n++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if want(s, &d) {
				return s
			}
			s.Apply(d.Options[pick(s, &d)])
		}
	}
	t.Fatal("no red-mirror game reached the scenario")
	return s
}

func lands(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, false) }

// burstCastable: seat 0's priority decision offers a Burst Lightning cast.
func burstCastable(s *dirac.State, d *dirac.Decision) bool {
	return d.Kind == dirac.DkPriority && d.Player == 0 && offers(s, d, func(a dirac.Action) bool { return castsCard(s, a, burst, false) })
}

// landAndBurst is seat 0 in its own first main phase offered a land play
// and a Burst Lightning cast: the stand-in bot's answer is the land.
func landAndBurst(s *dirac.State, d *dirac.Decision) bool {
	return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 &&
		offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand })
}

// upkeepWindow is seat 0 in its own upkeep with an empty stack, offered a
// Burst Lightning cast and a Firebrand activation.
func upkeepWindow(s *dirac.State, d *dirac.Decision) bool {
	return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepUpkeep && s.StackLen() == 0 &&
		offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkActivate && cardOf(s, a) == firebrand })
}

func timingOpts() Options {
	o := DefaultOptions()
	o.Sims, o.PriorTopK, o.OpponentNodes, o.OpponentMirror, o.ClosedList = 64, 0, false, false, false
	o.Kinds = AllKinds()
	return o
}

// rootHas reports whether a candidate of the last search plays an action f
// accepts (its first action).
func rootHas(sr *Searcher, res *Result, f func(dirac.Action) bool) bool {
	for c := 0; c < res.N; c++ {
		if f(dirac.Action(sr.RootCand(c)[0])) {
			return true
		}
	}
	return false
}

// passBot passes at every priority decision (as the heuristic bot port does
// on its own turn outside a main phase) and is the stand-in elsewhere.
type passBot struct{}

// otherBot answers a priority decision with its first option that is
// neither pass nor a Burst Lightning cast (else pass), so a cands= cap of
// two keeps it and pass and cuts every Burst cast (the CardSimsForce tests
// on a root with no land play).
type otherBot struct{}

func (otherBot) Pick(s *dirac.State, d *dirac.Decision) int {
	if d.Kind == dirac.DkPriority {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() != dirac.AkPass && !castsCard(s, a, burst, false) {
				return i
			}
		}
	}
	return passBot{}.Pick(s, d)
}

func (passBot) Pick(s *dirac.State, d *dirac.Decision) int {
	if d.Kind == dirac.DkPriority {
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == dirac.AkPass {
				return i
			}
		}
	}
	return Heuristic{}.Pick(s, d)
}

// cardPrior is a network stand-in whose logits rank by card: Burst
// Lightning's casts low, pass high, everything else 0.
type cardPrior struct{}

func (cardPrior) HasPrior() bool { return true }
func (cardPrior) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	return HeuristicLeaf{}.Value(s, d, seat)
}
func (cardPrior) Priors(s *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N); i++ {
		switch a := d.Options[i]; {
		case castsCard(s, a, burst, false):
			out[i] = -8
		case a.Kind() == dirac.AkPass:
			out[i] = 4
		default:
			out[i] = 0
		}
	}
}

// TestHoldUpkeepDropsTheCasts: seat 0's own upkeep with an empty stack,
// Burst Lightning castable and a Firebrand activation offered. With the
// bot passing, the held root is the unheld one minus every cast (pass and
// the activation stay), HeldUpkeep 1; exempting Burst Lightning restores
// the unheld search exactly. With the stand-in bot, whose answer there is
// the cast, the held root is not searched and pass is played
// (HeldUpkeepBot 1), on the Sims 0 path too; without the rule the cast is
// searched as candidate 0.
func TestHoldUpkeepDropsTheCasts(t *testing.T) {
	s := redRoot(t, func(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, true) }, upkeepWindow)
	isCast := func(a dirac.Action) bool { return a.Kind() == dirac.AkCast }
	o := timingOpts()
	off := NewSearcher(o, HeuristicLeaf{}, passBot{})
	ro := *off.Search(s, 3)
	if ro.Stats.Searched != 1 || !rootHas(off, &ro, isCast) || ro.Stats.HeldUpkeep != 0 {
		t.Fatalf("off: searched %d, casts among the candidates %v, HeldUpkeep %d", ro.Stats.Searched, rootHas(off, &ro, isCast), ro.Stats.HeldUpkeep)
	}
	var want []Key
	for c := 0; c < ro.N; c++ {
		if !isCast(dirac.Action(off.RootCand(c)[0])) {
			want = append(want, ro.Keys[c])
		}
	}
	o.HoldUpkeep = true
	on := NewSearcher(o, HeuristicLeaf{}, passBot{})
	rh := *on.Search(s, 3)
	if rh.Stats.Searched != 1 || rh.Stats.HeldUpkeep != 1 || rh.Stats.HeldUpkeepBot != 0 {
		t.Fatalf("on: searched %d HeldUpkeep %d HeldUpkeepBot %d, want 1 1 0", rh.Stats.Searched, rh.Stats.HeldUpkeep, rh.Stats.HeldUpkeepBot)
	}
	if rh.N != len(want) || len(want) < 2 {
		t.Fatalf("on: %d candidates, want the %d uncast ones", rh.N, len(want))
	}
	for c := range want {
		if rh.Keys[c] != want[c] {
			t.Fatalf("on: candidate %d key differs from the unheld root's", c)
		}
	}
	if !rootHas(on, &rh, func(a dirac.Action) bool { return a.Kind() == dirac.AkActivate }) {
		t.Fatal("on: the Firebrand activation was dropped")
	}
	o.HoldUpkeepExempt[0] = burst
	ex := NewSearcher(o, HeuristicLeaf{}, passBot{})
	if re := *ex.Search(s, 3); !sameSearch(&re, &ro) || re.Stats.HeldUpkeep != 0 {
		t.Fatalf("exempt: the search differs from the unheld one (HeldUpkeep %d)", re.Stats.HeldUpkeep)
	}

	// the stand-in bot casts Burst Lightning in its upkeep
	o = timingOpts()
	plain := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	rp := plain.Search(s, 3)
	if rp.Stats.Searched != 1 || !isCast(dirac.Action(plain.RootCand(0)[0])) {
		t.Fatalf("off, stand-in bot: searched %d, candidate 0 %v; want its cast searched", rp.Stats.Searched, dirac.Action(plain.RootCand(0)[0]))
	}
	o.HoldUpkeep = true
	for _, sims := range []int{64, 0} {
		o.Sims = sims
		held := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
		r := held.Search(s, 3)
		wantHeld := 1
		if sims == 0 {
			wantHeld = 0 // no enumeration on the Sims 0 path
		}
		if r.Stats.Searched != 0 || r.NActs != 1 || r.Acts[0].Kind() != dirac.AkPass || r.Stats.HeldUpkeepBot != 1 || r.Stats.HeldUpkeep != wantHeld {
			t.Fatalf("on, stand-in bot, sims %d: searched %d plays %v HeldUpkeepBot %d HeldUpkeep %d; want pass unsearched, 1, %d",
				sims, r.Stats.Searched, r.Acts[0].Kind(), r.Stats.HeldUpkeepBot, r.Stats.HeldUpkeep, wantHeld)
		}
	}
}

// TestHoldUpkeepDrawStep: the draw step is inside the window as the
// upkeep is: seat 0's own draw step with an empty stack holds its casts.
func TestHoldUpkeepDrawStep(t *testing.T) {
	ownDraw := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepDraw && s.StackLen() == 0
	}
	s := redRoot(t, lands, ownDraw)
	isCast := func(a dirac.Action) bool { return a.Kind() == dirac.AkCast }
	o := timingOpts()
	o.Sims = 8
	off := NewSearcher(o, HeuristicLeaf{}, passBot{})
	ro := *off.Search(s, 4)
	o.HoldUpkeep = true
	on := NewSearcher(o, HeuristicLeaf{}, passBot{})
	rh := *on.Search(s, 4)
	if ro.Stats.Searched != 1 || !rootHas(off, &ro, isCast) || rh.Stats.HeldUpkeep != 1 || (rh.Stats.Searched == 1 && rootHas(on, &rh, isCast)) {
		t.Fatalf("own draw: off searched %d with casts %v; on HeldUpkeep %d searched %d", ro.Stats.Searched, rootHas(off, &ro, isCast), rh.Stats.HeldUpkeep, rh.Stats.Searched)
	}
}

// TestHoldUpkeepWindow: outside the window the root's candidates are the
// unheld ones -- seat 0's own main phase, its upkeep with the opponent's
// spell on the stack, and its priority in the opponent's upkeep -- and
// HeldUpkeep stays 0.
func TestHoldUpkeepWindow(t *testing.T) {
	ownMain := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 &&
			!offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand })
	}
	oppUpkeep := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 1 && s.StepNow() == kop.StepUpkeep && s.StackLen() == 0
	}
	// seat 1 casts Burst Lightning in seat 0's upkeep, so seat 0 answers
	// with the stack non-empty
	respond := func(s *dirac.State, d *dirac.Decision) int {
		if d.Kind == dirac.DkPriority && d.Player == 1 && s.ActivePlayer() == 0 && s.StepNow() == kop.StepUpkeep && s.StackLen() == 0 {
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; castsCard(s, a, burst, false) && a.Mode() == 0 {
					return i
				}
			}
		}
		return scriptPick(s, d, false)
	}
	stacked := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepUpkeep && s.StackLen() > 0
	}
	for _, c := range []struct {
		name string
		pick func(s *dirac.State, d *dirac.Decision) int
		want func(s *dirac.State, d *dirac.Decision) bool
	}{{"own main", lands, ownMain}, {"own upkeep, stack", respond, stacked}, {"opponent's upkeep", lands, oppUpkeep}} {
		s := redRoot(t, c.pick, c.want)
		o := timingOpts()
		o.Sims = 8
		ro := *NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 5)
		o.HoldUpkeep = true
		rh := *NewSearcher(o, HeuristicLeaf{}, passBot{}).Search(s, 5)
		if ro.Stats.Searched != 1 || rh.N != ro.N || rh.Keys != ro.Keys || rh.Stats.HeldUpkeep != 0 || rh.Stats.HeldUpkeepBot != 0 {
			t.Fatalf("%s: held root %d candidates (HeldUpkeep %d), unheld %d (searched %d); want the same root", c.name, rh.N, rh.Stats.HeldUpkeep, ro.N, ro.Stats.Searched)
		}
	}
}

// stackSpy is the heuristic leaf counting the leaves at which seat 0's own
// Burst Lightning is the bottom of the stack in its own upkeep or draw
// step: a cast made with the stack empty, which the rule holds, played
// inside a simulation.
type stackSpy struct{ hits *int }

func (sp stackSpy) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	if st := s.StepNow(); s.ActivePlayer() == 0 && (st == kop.StepUpkeep || st == kop.StepDraw) && s.StackLen() > 0 {
		// the bottom entry: cast with the stack empty (a response to the
		// opponent's spell sits above it, and is allowed)
		if _, _, card, ctrl := s.StackEntry(0); ctrl == 0 && s.Table().Cards[card].Name == burst {
			*sp.hits++
		}
	}
	return HeuristicLeaf{}.Value(s, d, seat)
}

func (stackSpy) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N); i++ {
		out[i] = 0
	}
}

// TestHoldUpkeepPassesTheWalkBot: inside a simulation the stand-in bot
// casts Burst Lightning in seat 0's own upkeep, an answer outside the held
// candidates, so the decision is no point and pass is played in its place:
// with the rule on no leaf sees seat 0's cast on the stack in its own
// upkeep or draw (the unmirrored opponent's decisions, points under
// OpponentNodes, would see it), and without the rule some leaf does.
func TestHoldUpkeepPassesTheWalkBot(t *testing.T) {
	oppEnd := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 1 && s.StepNow() == kop.StepEnd && s.StackLen() == 0
	}
	s := redRoot(t, lands, oppEnd)
	for _, hold := range []bool{false, true} {
		o := timingOpts()
		o.Sims, o.HoldUpkeep, o.OpponentNodes = 300, hold, true
		hits := 0
		r := NewSearcher(o, stackSpy{&hits}, Heuristic{}).Search(s, 11)
		if hold && (hits != 0 || r.Stats.HeldUpkeepWalk == 0) || !hold && hits == 0 {
			t.Fatalf("hold %v: %d leaves saw seat 0's upkeep cast (HeldUpkeepWalk %d)", hold, hits, r.Stats.HeldUpkeepWalk)
		}
	}
}

// TestHoldUpkeepHoldsTheWalk: from seat 0's priority in the opponent's end
// step the walk reaches seat 0's own upkeep, where the rule holds its
// Burst Lightning (HeldUpkeepWalk), and the tree differs; from seat 0's own
// end step a mirrored opponent's walk reaches the opponent's own upkeep
// and draw (HeldUpkeepOpp), and an unmirrored opponent is never held.
func TestHoldUpkeepHoldsTheWalk(t *testing.T) {
	oppEnd := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 1 && s.StepNow() == kop.StepEnd && s.StackLen() == 0
	}
	s := redRoot(t, lands, oppEnd)
	o := timingOpts()
	o.Sims = 200
	ro := *NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 9)
	o.HoldUpkeep = true
	rh := *NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 9)
	if ro.Stats.HeldUpkeepWalk != 0 || rh.Stats.HeldUpkeepWalk == 0 || rh.Stats.HeldUpkeepOpp != 0 {
		t.Fatalf("opponent's end step: HeldUpkeepWalk %d off, %d on (opp %d); want 0, > 0, 0", ro.Stats.HeldUpkeepWalk, rh.Stats.HeldUpkeepWalk, rh.Stats.HeldUpkeepOpp)
	}
	if rh.N != ro.N || rh.Keys != ro.Keys || sameSearch(&rh, &ro) {
		t.Fatal("opponent's end step: the root should be the same and the tree differ")
	}

	ownEnd := func(s *dirac.State, d *dirac.Decision) bool {
		return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepEnd && s.StackLen() == 0
	}
	s = redRoot(t, lands, ownEnd)
	for _, c := range []struct {
		nodes, mirror, held bool
	}{{true, true, true}, {true, false, false}, {false, false, false}} {
		o := timingOpts()
		o.Sims, o.HoldUpkeep, o.OpponentNodes, o.OpponentMirror = 200, true, c.nodes, c.mirror
		r := NewSearcher(o, HeuristicLeaf{}, Heuristic{}).Search(s, 9)
		if got := r.Stats.HeldUpkeepOpp > 0; got != c.held {
			t.Fatalf("own end step, oppnodes %v oppfull %v: HeldUpkeepOpp %d, want held %v", c.nodes, c.mirror, r.Stats.HeldUpkeepOpp, c.held)
		}
	}
}

// postDrop is seat 0 in its own main phase after its land drop with
// Burst Lightning castable: the stand-in bot casts it.
func postDrop(s *dirac.State, d *dirac.Decision) bool {
	return burstCastable(s, d) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 &&
		!offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand })
}

// TestCardSimsBoostsTheBudget: a root whose candidates cast Burst
// Lightning runs csims's 150 simulations whatever SeatSearch chose -- the
// base 20, a ksims= entry, a playout-cap cheap budget (the decision stays
// cheap and unrecorded) or a decided one (its weight kept); a root
// without the card runs its own budget; a land play is no cast; the tree
// is preallocated for the largest entry.
func TestCardSimsBoostsTheBudget(t *testing.T) {
	s := redRoot(t, lands, postDrop)
	var d dirac.Decision
	s.StepInto(&d)
	o := timingOpts()
	o.Sims = 20
	o.CardSims[0] = CardBudget{Name: "Mountain", Sims: 300} // land plays never match
	o.CardSims[1] = CardBudget{Name: burst, Sims: 150}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	if MaxSims(&o) != 300 {
		t.Fatalf("MaxSims %d, want 300", MaxSims(&o))
	}
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	if len(sr.t.nodes) != 302 {
		t.Fatalf("tree of %d nodes, want MaxSims+2", len(sr.t.nodes))
	}
	r := sr.Search(s, 1)
	if r.Stats.Searched != 1 || r.Sims != 150 || r.Stats.Simulations != 150 || r.Stats.CardBoosted != 1 {
		t.Fatalf("boosted: searched %d sims %d ran %d CardBoosted %d; want 150", r.Stats.Searched, r.Sims, r.Stats.Simulations, r.Stats.CardBoosted)
	}
	// a root with no listed card among its candidates (a Mountain is
	// played, never cast): its own budget
	o2 := o
	o2.CardSims[1] = CardBudget{}
	r2 := NewSearcher(o2, HeuristicLeaf{}, Heuristic{}).Search(s, 1)
	if r2.Sims != 20 || r2.Stats.CardBoosted != 0 {
		t.Fatalf("unlisted: sims %d CardBoosted %d, want 20 and 0", r2.Sims, r2.Stats.CardBoosted)
	}
	// over a ksims= entry, a cheap budget and a decided one
	o.SimsByKind[kindPriority] = 30
	var st SeatState
	k := NewSearcher(o, HeuristicLeaf{}, Heuristic{}).SeatSearch(s, &d, 1, &st)
	if k.Sims != 150 {
		t.Fatalf("ksims: sims %d, want 150", k.Sims)
	}
	o.FullProb, o.CheapSims = 1e-12, 5
	st = SeatState{}
	c := NewSearcher(o, HeuristicLeaf{}, Heuristic{}).SeatSearch(s, &d, 1, &st)
	if c.Sims != 150 || !c.Cheap || c.Record {
		t.Fatalf("cheap: sims %d cheap %v record %v, want 150, a cheap unrecorded search", c.Sims, c.Cheap, c.Record)
	}
	o.FullProb, o.CheapSims = 0, 0
	o.DecidedHigh, o.DecidedN, o.DecidedSims, o.DecidedWeight = 0.9, 1, 7, 0.1
	st = SeatState{side: 1, run: 3}
	dd := NewSearcher(o, HeuristicLeaf{}, Heuristic{}).SeatSearch(s, &d, 1, &st)
	if dd.Sims != 150 || !dd.Decided || dd.Weight != 0.1 {
		t.Fatalf("decided: sims %d decided %v weight %g, want 150, decided at 0.1", dd.Sims, dd.Decided, dd.Weight)
	}
}

// TestCardSimsForce: with landsearch on, the land-and-Burst root under
// topk=2 and a prior that ranks Burst Lightning last keeps the land and
// pass, so the card's cast is cut and nothing is boosted; csimsforce adds
// its casts back past the cut (CardForced), boosts the search, keeps the
// land as candidate 0 and renormalises the prior. Without a network prior
// the cands= cap is the cut. A held cast is never forced back.
func TestCardSimsForce(t *testing.T) {
	s := redRoot(t, lands, postDrop)
	isBurst := func(a dirac.Action) bool { return castsCard(s, a, burst, false) }
	o := timingOpts()
	o.Sims, o.PriorTopK = 20, 2
	o.CardSims[0] = CardBudget{Name: burst, Sims: 90}
	cut := NewSearcher(o, cardPrior{}, otherBot{})
	r := *cut.Search(s, 2)
	if r.N != 2 || rootHas(cut, &r, isBurst) || r.Sims != 20 || r.Stats.CardBoosted != 0 {
		t.Fatalf("cut: %d candidates (Burst %v), sims %d, CardBoosted %d; want the land and pass at 20", r.N, rootHas(cut, &r, isBurst), r.Sims, r.Stats.CardBoosted)
	}
	land := dirac.Action(cut.RootCand(0)[0])
	o.CardSimsForce = true
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name  string
		eval  Evaluator
		topk  int
		limit int
	}{{"topk", cardPrior{}, 2, 6}, {"cands", HeuristicLeaf{}, 0, 2}} {
		o.PriorTopK, o.Limit = c.topk, c.limit
		f := NewSearcher(o, c.eval, otherBot{})
		r := *f.Search(s, 2)
		if r.Stats.CardForced < 1 || r.N != 2+r.Stats.CardForced || !rootHas(f, &r, isBurst) || r.Sims != 90 || r.Stats.CardBoosted != 1 {
			t.Fatalf("%s: %d candidates, CardForced %d, sims %d; want the cut's two plus the Burst casts at 90", c.name, r.N, r.Stats.CardForced, r.Sims)
		}
		if dirac.Action(f.RootCand(0)[0]) != land {
			t.Fatalf("%s: candidate 0 moved", c.name)
		}
		sum := 0.0
		for i := 0; i < r.N; i++ {
			sum += r.Prior[i]
		}
		if sum < 0.999999 || sum > 1.000001 {
			t.Fatalf("%s: prior sums to %g", c.name, sum)
		}
	}
	// the upkeep window: HoldUpkeep drops the cast and force never adds it back
	u := redRoot(t, func(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, true) }, upkeepWindow)
	o = timingOpts()
	o.HoldUpkeep, o.CardSimsForce = true, true
	o.CardSims[0] = CardBudget{Name: burst, Sims: 90}
	h := NewSearcher(o, HeuristicLeaf{}, passBot{})
	rh := *h.Search(u, 2)
	if rootHas(h, &rh, func(a dirac.Action) bool { return a.Kind() == dirac.AkCast }) || rh.Stats.CardForced != 0 || rh.Sims != o.Sims {
		t.Fatalf("held: a cast among the candidates (forced %d, sims %d)", rh.Stats.CardForced, rh.Sims)
	}
}

// TestTimingOptionsValidate: the card-name lists' rules.
func TestTimingOptionsValidate(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(o *Options)
	}{
		{"csims without sims", func(o *Options) { o.CardSims[0] = CardBudget{Name: burst} }},
		{"csims sims without a name", func(o *Options) { o.CardSims[0] = CardBudget{Sims: 5} }},
		{"csims gap", func(o *Options) { o.CardSims[1] = CardBudget{Name: burst, Sims: 5} }},
		{"csims twice", func(o *Options) {
			o.CardSims[0], o.CardSims[1] = CardBudget{Name: burst, Sims: 5}, CardBudget{Name: burst, Sims: 9}
		}},
		{"csimsforce alone", func(o *Options) { o.CardSimsForce = true }},
		{"exempt without holdupkeep", func(o *Options) { o.HoldUpkeepExempt[0] = burst }},
		{"exempt gap", func(o *Options) { o.HoldUpkeep, o.HoldUpkeepExempt[2] = true, burst }},
		{"exempt twice", func(o *Options) {
			o.HoldUpkeep, o.HoldUpkeepExempt[0], o.HoldUpkeepExempt[1] = true, burst, burst
		}},
	} {
		o := DefaultOptions()
		c.set(&o)
		if err := o.Validate(); err == nil {
			t.Errorf("%s: accepted", c.name)
		}
	}
	o := DefaultOptions()
	o.HoldUpkeep, o.CardSimsForce = true, true
	o.HoldUpkeepExempt[0], o.HoldUpkeepExempt[1] = "Opt", burst
	o.CardSims[0], o.CardSims[1] = CardBudget{Name: burst, Sims: 3200}, CardBudget{Name: firebrand, Sims: 400}
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
}

// TestTimingOptionsZeroAllocs: a search with every timing option on
// allocates nothing (a self-play host's hot loop).
func TestTimingOptionsZeroAllocs(t *testing.T) {
	o := timingOpts()
	o.Sims, o.HoldUpkeep, o.CardSimsForce = 32, true, true
	o.HoldUpkeepExempt[0] = firebrand
	o.CardSims[0] = CardBudget{Name: burst, Sims: 48}
	o.OpponentNodes, o.OpponentMirror = true, true
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	for _, want := range []func(s *dirac.State, d *dirac.Decision) bool{landAndBurst, upkeepWindow} {
		s := redRoot(t, func(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, true) }, want)
		if allocs := testing.AllocsPerRun(5, func() { sr.Search(s, 1) }); allocs != 0 {
			t.Fatalf("allocs per search = %v", allocs)
		}
	}
}

// TestCardSimsMatches: a CardSims entry matches an activation as it does a
// cast -- at seat 0's upkeep Fanatical Firebrand is only activated (a
// creature is never cast there), and its entry boosts the search -- and
// with two listed cards among the candidates the larger entry's budget
// wins, whichever card holds it and in either list order.
func TestCardSimsMatches(t *testing.T) {
	s := redRoot(t, func(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, true) }, upkeepWindow)
	o := timingOpts()
	o.Sims, o.Limit = 20, MaxCands
	plain := NewSearcher(o, HeuristicLeaf{}, passBot{})
	r := *plain.Search(s, 2)
	if !rootHas(plain, &r, func(a dirac.Action) bool { return castsCard(s, a, firebrand, true) }) ||
		rootHas(plain, &r, func(a dirac.Action) bool { return castsCard(s, a, firebrand, false) }) ||
		!rootHas(plain, &r, func(a dirac.Action) bool { return castsCard(s, a, burst, false) }) {
		t.Fatal("the upkeep root should offer the Firebrand activation, no Firebrand cast, and a Burst Lightning cast")
	}
	for _, c := range []struct {
		entries []CardBudget
		want    int
	}{
		{[]CardBudget{{firebrand, 77}}, 77},
		{[]CardBudget{{burst, 40}, {firebrand, 70}}, 70},
		{[]CardBudget{{firebrand, 70}, {burst, 40}}, 70},
		{[]CardBudget{{burst, 70}, {firebrand, 40}}, 70},
		{[]CardBudget{{firebrand, 40}, {burst, 70}}, 70},
	} {
		oc := o
		copy(oc.CardSims[:], c.entries)
		r := NewSearcher(oc, HeuristicLeaf{}, passBot{}).Search(s, 2)
		if r.Sims != c.want || r.Stats.Simulations != c.want || r.Stats.CardBoosted != 1 {
			t.Errorf("%v: sims %d ran %d CardBoosted %d, want %d", c.entries, r.Sims, r.Stats.Simulations, r.Stats.CardBoosted, c.want)
		}
	}
}

// TestHoldUpkeepBeforeTheCap: the held casts leave the candidates before
// the cands= cap, so they take no slot -- at seat 0's upkeep with cands=2,
// where the decision lists a Burst Lightning cast before the Firebrand
// activation, the unheld root is pass and the cast, the held one pass and
// the activation.
func TestHoldUpkeepBeforeTheCap(t *testing.T) {
	s := redRoot(t, func(s *dirac.State, d *dirac.Decision) int { return scriptPick(s, d, true) }, upkeepWindow)
	var d dirac.Decision
	s.StepInto(&d)
	firstCast, act := -1, -1
	for i := 0; i < int(d.N); i++ {
		switch a := d.Options[i]; {
		case a.Kind() == dirac.AkCast && firstCast < 0:
			firstCast = i
		case castsCard(s, a, firebrand, true) && a.Kind() == dirac.AkActivate && act < 0:
			act = i
		}
	}
	if firstCast < 0 || act < firstCast {
		t.Fatalf("the upkeep decision lists the first cast at %d and the activation at %d; want the cast first", firstCast, act)
	}
	o := timingOpts()
	o.Sims, o.Limit = 16, 2
	off := NewSearcher(o, HeuristicLeaf{}, passBot{})
	ro := *off.Search(s, 2)
	if ro.N != 2 || dirac.Action(off.RootCand(1)[0]) != d.Options[firstCast] {
		t.Fatalf("off: %d candidates, 1 = %v; want pass and the first cast", ro.N, dirac.Action(off.RootCand(1)[0]))
	}
	o.HoldUpkeep = true
	on := NewSearcher(o, HeuristicLeaf{}, passBot{})
	rh := *on.Search(s, 2)
	if rh.Stats.Searched != 1 || rh.N != 2 || dirac.Action(on.RootCand(0)[0]).Kind() != dirac.AkPass || dirac.Action(on.RootCand(1)[0]) != d.Options[act] {
		t.Fatalf("held: searched %d, %d candidates; want pass and the Firebrand activation", rh.Stats.Searched, rh.N)
	}
}

// TestCardSimsForcePrior: the forced candidates enter the root's prior as
// any candidate does -- under a ranked prior the softmax of cardPrior's
// logits over every kept candidate (the forced casts' -8 included, so each
// has a prior above 0), with no network prior uniform -- and each forced
// candidate's key is the key the same cast has where no cut drops it.
func TestCardSimsForcePrior(t *testing.T) {
	s := redRoot(t, lands, postDrop)
	var d dirac.Decision
	s.StepInto(&d)
	logit := func(a dirac.Action) float64 {
		var out [dirac.MaxOptions]float32
		cardPrior{}.Priors(s, &d, nil, out[:d.N])
		for i := 0; i < int(d.N); i++ {
			if d.Options[i] == a {
				return float64(out[i])
			}
		}
		t.Fatalf("%v is no option", a)
		return 0
	}
	o := timingOpts()
	o.Sims, o.Limit = 20, MaxCands
	uncut := NewSearcher(o, HeuristicLeaf{}, otherBot{})
	ru := *uncut.Search(s, 2)
	keyOf := map[dirac.Action]Key{}
	for c := 0; c < ru.N; c++ {
		keyOf[dirac.Action(uncut.RootCand(c)[0])] = ru.Keys[c]
	}
	o.CardSims[0], o.CardSimsForce = CardBudget{Name: burst, Sims: 90}, true
	for _, c := range []struct {
		name        string
		eval        Evaluator
		topk, limit int
	}{{"topk", cardPrior{}, 2, 6}, {"cands", HeuristicLeaf{}, 0, 2}} {
		o.PriorTopK, o.Limit = c.topk, c.limit
		f := NewSearcher(o, c.eval, otherBot{})
		r := *f.Search(s, 2)
		if r.Stats.CardForced < 1 || r.N != 2+r.Stats.CardForced {
			t.Fatalf("%s: %d candidates, CardForced %d", c.name, r.N, r.Stats.CardForced)
		}
		z := 0.0
		for i := 0; i < r.N; i++ {
			z += math.Exp(logit(dirac.Action(f.RootCand(i)[0])))
		}
		for i := 0; i < r.N; i++ {
			a := dirac.Action(f.RootCand(i)[0])
			want := 1 / float64(r.N)
			if c.topk > 0 {
				want = math.Exp(logit(a)) / z
			}
			if r.Prior[i] <= 0 || math.Abs(r.Prior[i]-want) > 1e-9 {
				t.Errorf("%s: candidate %d (%v) prior %g, want %g", c.name, i, a.Kind(), r.Prior[i], want)
			}
			if k, ok := keyOf[a]; !ok || k != r.Keys[i] {
				t.Errorf("%s: candidate %d (%v) key %x, the uncut root's %x (present %v)", c.name, i, a.Kind(), r.Keys[i], k, ok)
			}
		}
	}
}

// TestTreeBytesPerSim pins the tree's size per simulation that TreeBytes
// and the CardSims memory note quote (cardsims.go): a
// node and MaxCands edges per MaxSims+2, preallocated per Searcher.
func TestTreeBytesPerSim(t *testing.T) {
	if n := unsafe.Sizeof(node{}) + MaxCands*unsafe.Sizeof(edge{}); n != 4648 {
		t.Fatalf("%d bytes per simulation, the docs say 4,648: update them", n)
	}
	o := timingOpts()
	o.CardSims[0] = CardBudget{Name: burst, Sims: 3200}
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	if len(sr.t.nodes) != 3202 || len(sr.t.edges) != 3202*MaxCands {
		t.Fatalf("%d nodes and %d edges for MaxSims 3200", len(sr.t.nodes), len(sr.t.edges))
	}
	if b := TreeBytes(&o); b != 3202*4648 {
		t.Fatalf("TreeBytes %d, want %d", b, 3202*4648)
	}
}

// landPrior is a network stand-in whose logits favour the land play, so
// the land draws the visits whatever its value.
type landPrior struct{}

func (landPrior) HasPrior() bool { return true }
func (landPrior) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	return HeuristicLeaf{}.Value(s, d, seat)
}
func (landPrior) Priors(s *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	for i := 0; i < int(d.N); i++ {
		out[i] = 0
		if d.Options[i].Kind() == dirac.AkPlayLand {
			out[i] = 2
		}
	}
}
