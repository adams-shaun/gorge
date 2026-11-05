package search

import (
	"math"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Tests for Options.WaitTie (timing.go), the "when tied, wait" final-move
// rule, and dirac State.InstantSpeed, which it reads the timing from. The
// scenarios are FDN games of a red deck of Mountains, Gorehorn Raider (a
// {4}{R} creature: sorcery-speed), Burst Lightning (an instant) and
// Fanatical Firebrand (a {R} creature with an instant-speed "{T},
// sacrifice" ability), played by a fixed script (scriptPick: lands,
// Firebrands with develop, else pass) to the decision a test needs. The
// card names only find the scenarios: the rule reads none.

var waitDeck = []cardset.Entry{{Name: "Mountain", Count: 17}, {Name: gorehorn, Count: 8}, {Name: burst, Count: 8}, {Name: firebrand, Count: 7}}

// optOf is the first option of d that f accepts, or -1.
func optOf(d *dirac.Decision, f func(dirac.Action) bool) int {
	for i := 0; i < int(d.N); i++ {
		if f(d.Options[i]) {
			return i
		}
	}
	return -1
}

func isPassAct(a dirac.Action) bool { return a.Kind() == dirac.AkPass }

// waitScene is a scenario root: seat 0's priority decision, stepped into.
type waitScene struct {
	name  string
	pick  func(s *dirac.State, d *dirac.Decision) int
	want  func(s *dirac.State, d *dirac.Decision) bool
	card  string // the move A: a cast of this card (or, act, an activation)
	act   bool
	wait  bool // laterWindow's answer: the rule may wait
	untap bool // its answer under the untap-step horizon (Options.WaitTieUntap)
}

func castable(s *dirac.State, d *dirac.Decision, name string, act bool) bool {
	return d.Kind == dirac.DkPriority && d.Player == 0 && offers(s, d, func(a dirac.Action) bool {
		return castsCard(s, a, name, act) && (!act || a.Kind() == dirac.AkActivate)
	})
}

// at is seat 0's priority decision offering card's move in step st of
// active's turn, with an empty stack (or, resp, an object of seat 1 on it).
func at(card string, act bool, active uint8, st kop.Step, resp bool) func(s *dirac.State, d *dirac.Decision) bool {
	return func(s *dirac.State, d *dirac.Decision) bool {
		if !castable(s, d, card, act) || s.ActivePlayer() != active || s.StepNow() != st {
			return false
		}
		if !resp {
			return s.StackLen() == 0
		}
		if s.StackLen() == 0 {
			return false
		}
		_, _, _, ctrl := s.StackEntry(s.StackLen() - 1)
		return ctrl == 1
	}
}

// noLand is want with no land play offered (the bot's answer is then no
// land, so the decision is searched without LandSearch).
func noLand(want func(s *dirac.State, d *dirac.Decision) bool) func(s *dirac.State, d *dirac.Decision) bool {
	return func(s *dirac.State, d *dirac.Decision) bool {
		return want(s, d) && !offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand })
	}
}

// attack is the scenario driver for the combat steps: a land when one is
// offered, else a creature cast (Firebrand or Raider) in the active
// player's main phase, every attacker it can declare at the opponent,
// every block it can declare, else pass or Done. It never casts Burst
// Lightning or activates an ability.
func attack(s *dirac.State, d *dirac.Decision) int {
	switch d.Kind {
	case dirac.DkPriority:
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == dirac.AkPlayLand {
				return i
			}
		}
		if d.Player == s.ActivePlayer() {
			for i := 0; i < int(d.N); i++ {
				if a := d.Options[i]; castsCard(s, a, firebrand, false) || castsCard(s, a, gorehorn, false) {
					return i
				}
			}
		}
	case dirac.DkAttackers:
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == dirac.AkAttack && a.Target() >= dirac.PlayerTarget0 {
				return i
			}
		}
	case dirac.DkBlockers:
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == dirac.AkBlock {
				return i
			}
		}
	}
	return scriptPick(s, d, false)
}

// waitScenes are the rule's timing cases: a sorcery-speed creature in the
// precombat main phase waits and in the postcombat one does not; an
// instant waits in every step before the end step, its own turn's or the
// opponent's (upkeep, draw, both main phases, each combat step the games
// reach: beginning of combat, declare attackers and blockers, combat
// damage, end of combat), and not in an end step or in response to a
// spell; an instant-speed ability waits as an instant does. Under the
// untap-step horizon (untap) the own end step's instant and ability wait
// too (the opponent's turn lies before the seat's untap step) and nothing
// else changes: the opponent's end step is the last window, a response
// never waits, and the sorcery-speed creature waits from main 1 only. The
// indexes of the first eleven are fixed (TestInstantSpeed reads 6 and 9).
// No game here reaches a cleanup priority window (a trigger or state-based
// action in cleanup); TestWaitTieSteps pins that step.
var waitScenes = []waitScene{
	{"sorcery-speed creature, own main 1", develop, noLand(at(gorehorn, false, 0, kop.StepMain1, false)), gorehorn, false, true, true},
	{"sorcery-speed creature, own main 2", lands, at(gorehorn, false, 0, kop.StepMain2, false), gorehorn, false, false, false},
	{"instant, own upkeep", lands, at(burst, false, 0, kop.StepUpkeep, false), burst, false, true, true},
	{"instant, own main 1", lands, at(burst, false, 0, kop.StepMain1, false), burst, false, true, true},
	{"instant, own main 2", lands, at(burst, false, 0, kop.StepMain2, false), burst, false, true, true},
	{"instant, own end step", lands, at(burst, false, 0, kop.StepEnd, false), burst, false, false, true},
	{"instant, opponent's upkeep, empty stack", lands, at(burst, false, 1, kop.StepUpkeep, false), burst, false, true, true},
	{"instant, opponent's main 1, empty stack", lands, at(burst, false, 1, kop.StepMain1, false), burst, false, true, true},
	{"instant, opponent's end step", lands, at(burst, false, 1, kop.StepEnd, false), burst, false, false, false},
	{"instant, in response to the opponent's spell", develop, at(burst, false, 1, kop.StepMain1, true), burst, false, false, false},
	{"instant-speed ability, own main 1", develop, at(firebrand, true, 0, kop.StepMain1, false), firebrand, true, true, true},
	{"instant, own draw step", lands, at(burst, false, 0, kop.StepDraw, false), burst, false, true, true},
	{"instant, own beginning of combat", lands, at(burst, false, 0, kop.StepBeginCombat, false), burst, false, true, true},
	{"instant, own declare attackers", attack, at(burst, false, 0, kop.StepAttackers, false), burst, false, true, true},
	{"instant, own declare blockers", attack, at(burst, false, 0, kop.StepBlockers, false), burst, false, true, true},
	{"instant, own combat damage", attack, at(burst, false, 0, kop.StepDamage, false), burst, false, true, true},
	{"instant, own end of combat", attack, at(burst, false, 0, kop.StepEndCombat, false), burst, false, true, true},
	{"instant, opponent's draw step, empty stack", lands, at(burst, false, 1, kop.StepDraw, false), burst, false, true, true},
	{"instant, opponent's beginning of combat, empty stack", lands, at(burst, false, 1, kop.StepBeginCombat, false), burst, false, true, true},
	{"instant, opponent's declare attackers, empty stack", attack, at(burst, false, 1, kop.StepAttackers, false), burst, false, true, true},
	{"instant, opponent's declare blockers, empty stack", attack, at(burst, false, 1, kop.StepBlockers, false), burst, false, true, true},
	{"instant, opponent's combat damage, empty stack", attack, at(burst, false, 1, kop.StepDamage, false), burst, false, true, true},
	{"instant, opponent's end of combat, empty stack", attack, at(burst, false, 1, kop.StepEndCombat, false), burst, false, true, true},
	{"instant, opponent's main 2, empty stack", lands, at(burst, false, 1, kop.StepMain2, false), burst, false, true, true},
	{"instant-speed ability, own end step", develop, at(firebrand, true, 0, kop.StepEnd, false), firebrand, true, false, true},
	{"instant-speed ability, opponent's end step", develop, at(firebrand, true, 1, kop.StepEnd, false), firebrand, true, false, false},
	{"instant-speed ability, own main 2", develop, at(firebrand, true, 0, kop.StepMain2, false), firebrand, true, true, true},
}

func (c *waitScene) root(t *testing.T) (*dirac.State, dirac.Action, dirac.Action) {
	t.Helper()
	s := deckRoot(t, waitDeck, c.pick, c.want)
	var d dirac.Decision
	s.StepInto(&d) // the decision itself (deckRoot's s is stepped into it)
	ai := optOf(&d, func(a dirac.Action) bool {
		return castsCard(s, a, c.card, c.act) && (!c.act || a.Kind() == dirac.AkActivate)
	})
	pi := optOf(&d, isPassAct)
	if ai < 0 || pi < 0 {
		t.Fatalf("%s: no %s move or no pass among %v", c.name, c.card, d.Options[:d.N])
	}
	return s, d.Options[ai], d.Options[pi]
}

// tieRes is a two-candidate root: candidate 0 plays a, candidate 1 pass,
// with those visits and Q values (the visit argmax VisitBest).
func tieRes(a, pass dirac.Action, va, vp int32, qa, qp float64) *Result {
	r := &Result{N: 2}
	r.CandOne[0], r.CandAct[0], r.CandOne[1], r.CandAct[1] = true, a, true, pass
	r.Visits[0], r.Visits[1], r.Q[0], r.Q[1] = va, vp, qa, qp
	if vp > va {
		r.VisitBest = 1
	}
	return r
}

// ruleAt runs waitTie at root s with options o on r's chosen move best.
func ruleAt(s *dirac.State, o Options, r *Result, best int) (int, Stats) {
	o.Sims = 4
	sr := NewSearcher(o, HeuristicLeaf{}, Heuristic{})
	sr.root.CopyFrom(s)
	return sr.waitTie(r, best), sr.St
}

// TestInstantSpeed pins dirac State.InstantSpeed, the rule's timing
// source, against dirac's own timing: in seat 0's precombat main
// phase (a sorcery-speed window) a land play and a creature cast are
// sorcery-speed, an instant and a creature's "{T}, sacrifice" ability
// instant-speed; outside a sorcery-speed window (the opponent's turn, a
// response) every option is; and it allocates nothing and leaves the
// decision as it was.
func TestInstantSpeed(t *testing.T) {
	main1 := func(s *dirac.State, d *dirac.Decision) bool {
		return castable(s, d, gorehorn, false) && castable(s, d, burst, false) && castable(s, d, firebrand, true) &&
			s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0
	}
	s := deckRoot(t, waitDeck, develop, main1)
	var d, after dirac.Decision
	s.StepInto(&d)
	want := map[string]bool{}
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		got := s.InstantSpeed(a)
		var w bool
		switch {
		case a.Kind() == dirac.AkPlayLand:
			w = false
		case castsCard(s, a, gorehorn, false), castsCard(s, a, firebrand, false) && a.Kind() == dirac.AkCast:
			w = false
		case castsCard(s, a, burst, false), a.Kind() == dirac.AkActivate, a.Kind() == dirac.AkPass:
			w = true
		default:
			continue
		}
		if got != w {
			t.Errorf("main 1: InstantSpeed(%v %s) = %v, want %v", a.Kind(), cardOf(s, a), got, w)
		}
		if a.Kind() == dirac.AkCast || a.Kind() == dirac.AkActivate {
			want[cardOf(s, a)] = true
		}
	}
	if !want[gorehorn] || !want[burst] || !want[firebrand] {
		t.Fatalf("main 1 options cover %v", want)
	}
	s.StepInto(&after)
	if after != d {
		t.Fatal("InstantSpeed changed the pending decision")
	}
	a := d.Options[optOf(&d, func(a dirac.Action) bool { return castsCard(s, a, gorehorn, false) })]
	if n := testing.AllocsPerRun(20, func() { s.InstantSpeed(a) }); n != 0 {
		t.Fatalf("InstantSpeed allocates %v", n)
	}
	for _, c := range []waitScene{waitScenes[6], waitScenes[9]} {
		s, _, _ := c.root(t)
		s.StepInto(&d)
		for i := 0; i < int(d.N); i++ {
			if !s.InstantSpeed(d.Options[i]) {
				t.Errorf("%s: option %v is not instant-speed", c.name, d.Options[i].Kind())
			}
		}
	}
}

// TestWaitTieTiming: at each scenario a tied move (dQ 0.005 < EPS 0.01,
// both well visited) waits exactly where laterWindow says the timing rules
// offer it again within the horizon: this turn (WaitTieUntap off) a
// sorcery-speed creature in main 1, not in main 2; an instant on either
// turn before the end step, not in an end step or in response to a spell;
// before the seat's next untap step (WaitTieUntap on) the same, and an
// instant-speed move in the seat's own end step too. Where it waits the
// move is pass, WaitTie and WaitTieEligible are 1 and Waited names the
// move; elsewhere the move stands and both counters are 0.
func TestWaitTieTiming(t *testing.T) {
	for _, untap := range []bool{false, true} {
		o := timingOpts()
		o.WaitTie, o.WaitTieUntap = 0.01, untap
		for _, c := range waitScenes {
			s, a, pass := c.root(t)
			wait := c.wait
			if untap {
				wait = c.untap
			}
			if got := laterWindow(s, a, 0, untap); got != wait {
				t.Errorf("%s (untap %v): laterWindow %v, want %v", c.name, untap, got, wait)
			}
			r := tieRes(a, pass, 60, 40, 0.505, 0.5)
			got, st := ruleAt(s, o, r, 0)
			switch {
			case wait && (got != 1 || st.WaitTie != 1 || st.WaitTieEligible != 1 || r.Waited != 1):
				t.Errorf("%s (untap %v): tied move -> candidate %d (WaitTie %d, eligible %d, Waited %d), want pass", c.name, untap, got, st.WaitTie, st.WaitTieEligible, r.Waited)
			case !wait && (got != 0 || st.WaitTie != 0 || st.WaitTieEligible != 0 || r.Waited != 0):
				t.Errorf("%s (untap %v): tied move -> candidate %d (WaitTie %d, eligible %d, Waited %d), want the move", c.name, untap, got, st.WaitTie, st.WaitTieEligible, r.Waited)
			}
		}
	}
}

// TestWaitTieUntapOutlasts: under the untap-step horizon the own end step
// waits only for a move that outlasts the turn (dirac State.OutlastsTurn):
// at the scene's root the real Burst Lightning cast does, and the same
// cast in a flashback or play-permission mode -- instant-speed, as every
// option of the end step's window is, but resting on what ends with the
// turn -- does not; earlier in the turn (the own upkeep) the mode changes
// nothing.
func TestWaitTieUntapOutlasts(t *testing.T) {
	scene := func(name string) waitScene {
		for _, c := range waitScenes {
			if c.name == name {
				return c
			}
		}
		t.Fatalf("no scene %q", name)
		return waitScene{}
	}
	end := scene("instant, own end step")
	s, a, _ := end.root(t)
	if !s.InstantSpeed(a) || !s.OutlastsTurn(a) || !laterWindow(s, a, 0, true) {
		t.Fatal("own end step: the hand cast does not wait under the untap horizon")
	}
	for _, mode := range []uint8{2, dirac.FeatCastMayPlay} {
		b := dirac.MakeAction(dirac.AkCast, a.Source(), a.Target(), mode)
		if !s.InstantSpeed(b) || s.OutlastsTurn(b) {
			t.Fatalf("mode %d: InstantSpeed %v OutlastsTurn %v, want true, false", mode, s.InstantSpeed(b), s.OutlastsTurn(b))
		}
		if laterWindow(s, b, 0, true) {
			t.Errorf("own end step, mode %d: a turn-bound cast waits into the opponent's turn", mode)
		}
	}
	up := scene("instant, own upkeep")
	u, a, _ := up.root(t)
	b := dirac.MakeAction(dirac.AkCast, a.Source(), a.Target(), 2)
	if !u.InstantSpeed(b) || u.OutlastsTurn(b) || !laterWindow(u, b, 0, true) || !laterWindow(u, b, 0, false) {
		t.Error("own upkeep: a turn-bound cast does not wait within the turn")
	}
}

// hasteRoot is seat 0's own end step with an empty stack, offered the
// "{T}: target creature gains haste until end of turn" ability of an
// Axgard Cavalry it cast this turn, which another Cavalry of its gave
// haste: the ability is offered only through that haste, since the
// creature is summoning-sick. Its driver plays a land when one is
// offered and, in seat 0's own turn, casts one Cavalry, then activates
// each other Cavalry's ability with the new one as the target; seat 1
// plays lands only. It returns the root, the new Cavalry's ability and
// pass.
func hasteRoot(t *testing.T) (*dirac.State, dirac.Action, dirac.Action) {
	t.Helper()
	const cavalry = "Axgard Cavalry"
	deck := []cardset.Entry{{Name: "Mountain", Count: 16}, {Name: cavalry, Count: 24}}
	fresh := dirac.NoObj // the Cavalry seat 0 cast this turn
	isFresh := func(a dirac.Action) bool {
		return a.Kind() == dirac.AkActivate && dirac.Obj(a.Source()) == fresh
	}
	pick := func(s *dirac.State, d *dirac.Decision) int {
		if s.ActivePlayer() != 0 {
			fresh = dirac.NoObj
			return scriptPick(s, d, false)
		}
		switch {
		case d.Kind == dirac.DkPriority && d.Player == 0 && s.StackLen() == 0:
			if i := optOf(d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand }); i >= 0 {
				return i
			}
			if fresh == dirac.NoObj {
				if i := optOf(d, func(a dirac.Action) bool { return castsCard(s, a, cavalry, false) }); i >= 0 {
					fresh = dirac.Obj(d.Options[i].Source())
					return i
				}
			} else if s.ObjZone(fresh) == kop.ZoneBattlefield && !s.HasKeyword(fresh, kop.KwHaste) {
				if i := optOf(d, func(a dirac.Action) bool { return a.Kind() == dirac.AkActivate && !isFresh(a) }); i >= 0 {
					return i
				}
			}
		case d.Kind == dirac.DkTarget && fresh != dirac.NoObj:
			if i := optOf(d, func(a dirac.Action) bool { return dirac.Obj(a.Target()) == fresh }); i >= 0 {
				return i
			}
		}
		return scriptPick(s, d, false)
	}
	want := func(s *dirac.State, d *dirac.Decision) bool {
		return d.Kind == dirac.DkPriority && d.Player == 0 && s.ActivePlayer() == 0 && s.StepNow() == kop.StepEnd &&
			s.StackLen() == 0 && fresh != dirac.NoObj && s.HasKeyword(fresh, kop.KwHaste) && offers(s, d, isFresh)
	}
	s := deckRoot(t, deck, pick, want)
	var d dirac.Decision
	s.StepInto(&d)
	return s, d.Options[optOf(&d, isFresh)], d.Options[optOf(&d, isPassAct)]
}

// TestWaitTieUntapSickHaste: under the untap-step horizon a tied {T}
// ability of a summoning-sick creature that has haste only until end of
// turn does not wait from the seat's own end step (dirac
// State.OutlastsTurn: the creature is still sick in the opponent's turn
// and the haste is gone, so the ability is not offered there), as under
// the turn horizon.
func TestWaitTieUntapSickHaste(t *testing.T) {
	s, a, pass := hasteRoot(t)
	if !s.InstantSpeed(a) || s.OutlastsTurn(a) || s.NextTurnPlayer() != 1 {
		t.Fatalf("InstantSpeed %v OutlastsTurn %v NextTurnPlayer %d, want true, false, 1", s.InstantSpeed(a), s.OutlastsTurn(a), s.NextTurnPlayer())
	}
	for _, untap := range []bool{false, true} {
		if laterWindow(s, a, 0, untap) {
			t.Errorf("untap %v: the sick creature's ability waits from the own end step", untap)
		}
		o := timingOpts()
		o.WaitTie, o.WaitTieUntap = 0.01, untap
		r := tieRes(a, pass, 60, 40, 0.505, 0.5)
		if got, st := ruleAt(s, o, r, 0); got != 0 || st.WaitTie != 0 || st.WaitTieEligible != 0 || r.Waited != 0 {
			t.Errorf("untap %v: tied ability -> candidate %d (WaitTie %d, eligible %d, Waited %d), want the ability", untap, got, st.WaitTie, st.WaitTieEligible, r.Waited)
		}
	}
}

// TestWaitTiePastTurn pins pastTurn, whether the untap-step horizon
// reaches past this turn: only on the seat's own turn, with the next turn
// the opponent's (no extra turn of the seat's coming) and the move
// outlasting the turn.
func TestWaitTiePastTurn(t *testing.T) {
	for _, c := range []struct {
		active, next, p uint8
		outlasts, want  bool
	}{
		{0, 1, 0, true, true},
		{1, 0, 1, true, true},
		{0, 1, 0, false, false}, // the move rests on what ends with the turn
		{0, 0, 0, true, false},  // an extra turn of the seat's: its untap step follows this turn
		{1, 0, 0, true, false},  // the opponent's turn: the seat's untap step follows it
		{1, 1, 0, true, false},  // the opponent's turn with its extra turn coming: not counted
	} {
		if got := pastTurn(c.active, c.next, c.p, c.outlasts); got != c.want {
			t.Errorf("pastTurn(active %d, next %d, seat %d, outlasts %v) = %v, want %v", c.active, c.next, c.p, c.outlasts, got, c.want)
		}
	}
}

// TestWaitTieSteps pins laterStep, the timing condition's step model, at
// every step of the turn, the untap and cleanup steps included (no game
// of TestWaitTieTiming reaches a cleanup priority window): a
// sorcery-speed move is offered again within the horizon only from the
// precombat main phase, past this turn or not; an instant-speed one from
// every step from the upkeep through the postcombat main phase, and not
// from the end step, the cleanup step or the untap step -- unless the
// horizon reaches past this turn (past: the untap-step horizon on the
// seat's own turn), when the end and cleanup steps wait too, the untap
// step (no priority) still not.
func TestWaitTieSteps(t *testing.T) {
	steps := []struct {
		st                     kop.Step
		name                   string
		sorcery, instant, past bool
	}{
		{kop.StepUntap, "untap", false, false, false},
		{kop.StepUpkeep, "upkeep", false, true, true},
		{kop.StepDraw, "draw", false, true, true},
		{kop.StepMain1, "main 1", true, true, true},
		{kop.StepBeginCombat, "beginning of combat", false, true, true},
		{kop.StepAttackers, "declare attackers", false, true, true},
		{kop.StepBlockers, "declare blockers", false, true, true},
		{kop.StepFirstStrikeDamage, "first-strike damage", false, true, true},
		{kop.StepDamage, "combat damage", false, true, true},
		{kop.StepEndCombat, "end of combat", false, true, true},
		{kop.StepMain2, "main 2", false, true, true},
		{kop.StepEnd, "end", false, false, true},
		{kop.StepCleanup, "cleanup", false, false, true},
	}
	for i, c := range steps {
		if c.st != kop.Step(i) {
			t.Fatalf("step %s is %d, want %d: dirac's step order changed", c.name, c.st, i)
		}
		for _, past := range []bool{false, true} {
			if got := laterStep(c.st, false, past); got != c.sorcery {
				t.Errorf("%s (past %v): sorcery-speed laterStep %v, want %v", c.name, past, got, c.sorcery)
			}
		}
		if got := laterStep(c.st, true, false); got != c.instant {
			t.Errorf("%s: instant-speed laterStep %v, want %v", c.name, got, c.instant)
		}
		if got := laterStep(c.st, true, true); got != c.past {
			t.Errorf("%s: instant-speed laterStep past the turn %v, want %v", c.name, got, c.past)
		}
	}
	if kop.NumSteps != kop.Step(len(steps)) {
		t.Fatalf("dirac has %d steps, %d listed", kop.NumSteps, len(steps))
	}
}

// TestWaitTieRule: the rule's value and visit conditions at a main-1
// creature cast (a root where the timing lets it wait): the tie is strict
// (dQ < EPS; a pass valued above the move ties too); the move and pass
// both need the RootQ floor, max(2, RootQFrac x the top count), RootQFrac
// 0.1 by default; a pass move, a land play or a root with no pass
// candidate are left alone; the rule waits from the move rootPick chose
// as from the visit argmax.
func TestWaitTieRule(t *testing.T) {
	s, a, pass := waitScenes[0].root(t)
	o := timingOpts()
	o.WaitTie = 1.0 / 128 // 0.0078125
	for _, c := range []struct {
		name   string
		va, vp int32
		qa, qp float64
		frac   float64
		best   int
		want   int
		elig   int
	}{
		{"tied", 60, 40, 0.505, 0.5, 0, 0, 1, 1}, // dQ 0.005
		{"pass valued above", 60, 40, 0.48, 0.5, 0, 0, 1, 1},
		{"dQ = EPS", 60, 40, 0.5 + 1.0/128, 0.5, 0, 0, 0, 1}, // EPS 1/128 below: exact in binary
		{"dQ just under EPS", 60, 40, 0.5 + 1.0/128 - 1.0/4096, 0.5, 0, 0, 1, 1},
		{"dQ > EPS", 60, 40, 0.53, 0.5, 0, 0, 0, 1},
		{"pass under the floor", 60, 5, 0.5, 0.5, 0, 0, 0, 0},
		{"pass at the floor", 60, 6, 0.5, 0.5, 0, 0, 1, 1},
		{"pass at a 0.05 floor", 60, 3, 0.5, 0.5, 0.05, 0, 1, 1},
		{"pass unvisited, the floor 2", 10, 0, 0.5, 0, 0, 0, 0, 0},
		{"pass once, the floor 2", 10, 1, 0.5, 0.5, 0, 0, 0, 0},
		{"pass twice, the floor 2", 10, 2, 0.5, 0.5, 0, 0, 1, 1},
		{"pass chosen", 40, 60, 0.5, 0.5, 0, 1, 1, 0},
		{"the move rootPick chose", 40, 60, 0.505, 0.5, 0, 0, 1, 1}, // not the visit argmax
		{"the move under the floor", 4, 60, 0.505, 0.5, 0, 0, 0, 0},
	} {
		oo := o
		oo.RootQFrac = c.frac
		r := tieRes(a, pass, c.va, c.vp, c.qa, c.qp)
		got, st := ruleAt(s, oo, r, c.best)
		if got != c.want || st.WaitTieEligible != c.elig || st.WaitTie != b2i(c.want != c.best) {
			t.Errorf("%s: candidate %d (eligible %d, WaitTie %d), want %d (eligible %d)", c.name, got, st.WaitTieEligible, st.WaitTie, c.want, c.elig)
		}
	}
	// not a cast or activation: a land play, and a root with no pass
	var d dirac.Decision
	ls, _, _ := (&waitScene{"main 1, a land", lands, func(s *dirac.State, d *dirac.Decision) bool {
		return castable(s, d, burst, false) && s.ActivePlayer() == 0 && s.StepNow() == kop.StepMain1 && s.StackLen() == 0 &&
			offers(s, d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand })
	}, burst, false, true, true}).root(t)
	ls.StepInto(&d)
	land := d.Options[optOf(&d, func(a dirac.Action) bool { return a.Kind() == dirac.AkPlayLand })]
	if got, st := ruleAt(ls, o, tieRes(land, d.Options[optOf(&d, isPassAct)], 60, 40, 0.5, 0.5), 0); got != 0 || st.WaitTieEligible != 0 {
		t.Errorf("a land play waited: %d", got)
	}
	r := tieRes(a, a, 60, 40, 0.5, 0.5)
	if got, st := ruleAt(s, o, r, 0); got != 0 || st.WaitTieEligible != 0 {
		t.Errorf("no pass candidate: %d", got)
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestWaitTieSearch: the rule in a real Search at the main-1 creature
// root, under the Uniform evaluator (every leaf 0.5: the Q values tie, the
// visits spread evenly and the argmax is the bot's answer, the Raider cast)
// and EPS 1 (every well-visited cast ties with pass). Against the same
// search without it, only the move changes: the keys, visits, Q values,
// prior, root value and visit target are the same, Choice is pass and
// Waited names the move the search chose; WaitTie off is the old search
// bit for bit (Waited 0, the counters 0). Under Sample (generation mode)
// the draw is never moved, and at a LandSearch land root the rule never
// applies.
func TestWaitTieSearch(t *testing.T) {
	s, _, _ := waitScenes[0].root(t)
	o := timingOptsLimit()
	off := *NewSearcher(o, Uniform{}, Heuristic{}).Search(s, 3)
	if off.Stats.WaitTie != 0 || off.Stats.WaitTieEligible != 0 || off.Waited != 0 {
		t.Fatalf("off: WaitTie %d eligible %d Waited %d", off.Stats.WaitTie, off.Stats.WaitTieEligible, off.Waited)
	}
	o.WaitTie = 1
	sr := NewSearcher(o, Uniform{}, Heuristic{})
	on := *sr.Search(s, 3)
	if !off.CandOne[off.Choice] || off.CandAct[off.Choice].Kind() != dirac.AkCast {
		t.Fatalf("off: the search chose %v, want a cast (the scenario is the rule's)", off.CandAct[off.Choice].Kind())
	}
	pass := -1
	for c := 0; c < on.N; c++ {
		if on.CandOne[c] && on.CandAct[c].Kind() == dirac.AkPass {
			pass = c
		}
	}
	if pass < 0 || float64(on.Visits[pass]) < rootFloor(&on, &o, on.VisitBest) {
		t.Fatalf("pass candidate %d with %v visits: the scenario does not reach the rule", pass, on.Visits[:on.N])
	}
	if on.Choice != pass || on.Waited != off.Choice+1 || on.Stats.WaitTie != 1 || on.Stats.WaitTieEligible != 1 || on.NActs != 1 || !isPassAct(on.Acts[0]) {
		t.Fatalf("on: choice %d (pass %d), Waited %d (want %d), WaitTie %d eligible %d, plays %v", on.Choice, pass, on.Waited, off.Choice+1, on.Stats.WaitTie, on.Stats.WaitTieEligible, on.Acts[0].Kind())
	}
	same := on
	same.Choice, same.NActs, same.Acts = off.Choice, off.NActs, off.Acts
	if !sameSearch(&same, &off) || on.VisitBest != off.VisitBest || *on.PolicyTarget() != *off.PolicyTarget() {
		t.Fatal("on: the search's statistics differ from the search without the rule")
	}
	// EPS small enough that nothing ties: the old search, the root eligible
	o.WaitTie = math.SmallestNonzeroFloat64
	if q := off.Q[off.Choice] - off.Q[pass]; q >= o.WaitTie {
		e := *NewSearcher(o, Uniform{}, Heuristic{}).Search(s, 3)
		if !sameSearch(&e, &off) || e.Waited != 0 || e.Stats.WaitTie != 0 || e.Stats.WaitTieEligible != 1 {
			t.Fatalf("EPS tiny (dQ %g): choice %d Waited %d WaitTie %d eligible %d", q, e.Choice, e.Waited, e.Stats.WaitTie, e.Stats.WaitTieEligible)
		}
	}
	// Sample: the draw stands
	o.WaitTie = 1
	o.Sample = true
	so := o
	so.WaitTie = 0
	smp := *NewSearcher(o, Uniform{}, Heuristic{}).Search(s, 3)
	ref := *NewSearcher(so, Uniform{}, Heuristic{}).Search(s, 3)
	if !sameSearch(&smp, &ref) || smp.Stats.WaitTieEligible != 0 || smp.Waited != 0 {
		t.Fatal("Sample: the rule moved the draw")
	}
}

// TestWaitTieUntapValidate: the untap-step horizon is WaitTie's, refused
// without it.
func TestWaitTieUntapValidate(t *testing.T) {
	var o Options
	o.WaitTieUntap = true
	if o.Validate() == nil {
		t.Fatal("WaitTieUntap without WaitTie accepted")
	}
	o.WaitTie = 0.02
	if err := o.Validate(); err != nil {
		t.Fatal(err)
	}
}
