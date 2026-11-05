package search

import (
	"errors"
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The instant-timing options, each off by default and the search exactly
// as without it when off:
//
//   - the final-move rules RootQ / RootLCB (rootPick), WaitTie, its
//     untap-step horizon WaitTieUntap (waitTie, laterWindow), the WaitSorc
//     margin and the PassMax line value;
//   - the candidate rules Defer (deferBlocked, deferScale), HoldUpkeep,
//     RaidHold and CardSims / CardSimsForce;
//   - the prior temperature PriorTemp;
//   - the :react scope (ReactRoot, ReactHorizon, ReactTarget) on top of
//     instwin.go's InstWinReact tracker.

// rootFloor is RootQ's eligibility floor at res's root: max(2, F x the
// visits of vb, the most-visited child), F = RootQFrac, not > 0 (NaN
// included) meaning 0.1.
func rootFloor(res *Result, o *Options, vb int) float64 {
	f := o.RootQFrac
	if !(f > 0) {
		f = 0.1
	}
	return max(2, f*float64(res.Visits[vb]))
}

// isLandRoot reports whether the root is a land-search land root (the bot's
// answer a land play searched as candidate 0), where the final-move rules
// keep the land unless a line has strictly more visits. Without a land
// search option a land-play bot answer is never searched. Search sets
// landRoot where it decides the root's exploration (landsearch.go).
func (sr *Searcher) isLandRoot() bool { return sr.landRoot }

// finalMove applies the final-move rules to the visit argmax best at the
// searched root (Search calls it when RootQ, RootLCB, WaitTie or WaitSorc
// is on): RootQ / RootLCB (rootPick), then the wait (waitTie). Neither
// moves a sampled pick.
func (sr *Searcher) finalMove(res *Result, d *dirac.Decision, best int) int {
	if sr.Opts.Sample {
		return best
	}
	land := sr.isLandRoot()
	if sr.Opts.RootQ || sr.Opts.RootLCB {
		best = rootPick(res, &sr.Opts, best, land)
	}
	if (sr.Opts.WaitTie > 0 || sr.Opts.WaitSorcOn && sr.Opts.WaitSorc > 0) && !land && d.Kind == dirac.DkPriority {
		best = sr.waitTie(res, best)
	}
	if sr.Opts.DeferQ > 0 && !land && res.Waited == 0 {
		best = sr.deferQ(res, best) // instkeys.go
	}
	return best
}

// rootPick is the RootQ/RootLCB final-move rule: among root children with
// visits >= max(2, RootQFrac*maxvisits), the argmax of Q (RootQ) or of
// the lower bound Q - z*sqrt(max(Q(1-Q),0.01)/n) (RootLCB, z = RootLCBZ);
// ties and an empty eligible set keep the argmax-visits child vb. land is
// a land root (candidate 0 the land): another child is eligible only with
// strictly more visits than the land. A RootQFrac or RootLCBZ that is not
// > 0 (NaN included) is the default.
func rootPick(res *Result, o *Options, vb int, land bool) int {
	minN := rootFloor(res, o, vb)
	z := o.RootLCBZ
	if !(z > 0) {
		z = 1.96
	}
	if float64(res.Visits[vb]) < minN {
		return vb
	}
	best, bs := vb, rootScore(res, o, vb, z)
	for i := 0; i < res.N; i++ {
		if i == vb || float64(res.Visits[i]) < minN || (land && i != 0 && res.Visits[i] <= res.Visits[0]) {
			continue
		}
		if s := rootScore(res, o, i, z); s > bs {
			best, bs = i, s
		}
	}
	return best
}

func rootScore(res *Result, o *Options, i int, z float64) float64 {
	q := res.Q[i]
	if !o.RootLCB {
		return q
	}
	n := float64(res.Visits[i])
	v := q * (1 - q)
	if v < 0.01 {
		v = 0.01
	}
	return q - z*math.Sqrt(v/n)
}

// waitTie is the "when tied, wait" final-move rule: at a
// searched priority root whose chosen move A is one action, a cast or an
// activation, with pass a candidate and both visited at least the RootQ
// floor (rootFloor), where the game's timing rules alone offer A again
// within the horizon (laterWindow), pass is played when Q(A) - Q(pass) <
// EPS. EPS is WaitTie for an instant-speed A and, under WaitSorcOn, WaitSorc
// for a sorcery-speed one ("waitsorc=EPS, the sorcery-speed
// margin, waittie= then the instant-speed one"; 0 never waits). Under
// PassMax, Q(pass) is the best line after pass (passLine). Only the move
// changes: visits, Q, prior and the recorded target are the search's
// (a host's :waittarget swaps two counts in the record). Stats.WaitTieEligible
// is 1 for a root that met every condition but the tie, WaitTie 1 when the
// rule played pass, and Result.Waited names A (1 + its candidate index).
func (sr *Searcher) waitTie(res *Result, best int) int {
	if best < 0 || best >= res.N || !res.CandOne[best] {
		return best
	}
	a := res.CandAct[best]
	if k := a.Kind(); k != dirac.AkCast && k != dirac.AkCastCasualty && k != dirac.AkActivate {
		return best
	}
	pass := -1
	for i := 0; i < res.N && pass < 0; i++ {
		if res.CandOne[i] && res.CandAct[i].Kind() == dirac.AkPass {
			pass = i
		}
	}
	if pass < 0 {
		return best
	}
	minN := rootFloor(res, &sr.Opts, res.VisitBest)
	if float64(res.Visits[pass]) < minN || float64(res.Visits[best]) < minN {
		return best
	}
	s := &sr.root
	if s.StackLen() != 0 {
		return best
	}
	instant := s.InstantSpeed(a)
	eps := sr.Opts.WaitTie
	if !instant && sr.Opts.WaitSorcOn {
		eps = sr.Opts.WaitSorc
	}
	if !(eps > 0) {
		return best
	}
	past := sr.Opts.WaitTieUntap && instant && pastTurn(s.ActivePlayer(), s.NextTurnPlayer(), sr.seat, s.OutlastsTurn(a))
	if !laterStep(s.StepNow(), instant, past) {
		return best
	}
	sr.St.WaitTieEligible = 1
	qp := res.Q[pass]
	if sr.Opts.PassMax {
		if v, ok := sr.passLine(pass); ok {
			qp = v
			sr.St.PassMaxUsed = 1
		}
	}
	if !(res.Q[best]-qp < eps) {
		return best
	}
	sr.St.WaitTie = 1
	res.Waited = best + 1
	return pass
}

// laterWindow reports whether priority option a of s's pending decision
// (seat p's) is offered again within the rule's horizon by the game's
// timing rules alone: never with a non-empty stack (a response), else as
// laterStep says for the step, a's speed (dirac State.InstantSpeed) and,
// under the untap-step horizon (untap), whether the horizon reaches past
// this turn (pastTurn).
func laterWindow(s *dirac.State, a dirac.Action, p uint8, untap bool) bool {
	if s.StackLen() != 0 {
		return false
	}
	instant := s.InstantSpeed(a)
	past := untap && instant && pastTurn(s.ActivePlayer(), s.NextTurnPlayer(), p, s.OutlastsTurn(a))
	return laterStep(s.StepNow(), instant, past)
}

// pastTurn reports whether the untap-step horizon reaches past this turn
// for seat p's instant-speed move: on p's own turn (active), when the next
// turn is the opponent's (next: not p) and nothing the move rests on ends
// with the turn (outlasts, State.OutlastsTurn). On the opponent's turn p's
// untap step follows this turn, so it never does.
func pastTurn(active, next, p uint8, outlasts bool) bool {
	return active == p && next != p && outlasts
}

// laterStep reports whether a move of that speed, offered on an empty
// stack in step st, is offered again within the horizon: a sorcery-speed
// move only from the precombat main phase, where the postcombat one
// follows; an instant-speed move from any step from the upkeep through
// the postcombat main phase, since the end step's fresh priority round
// reaches both players, and not from the end step or the cleanup step --
// unless past (the untap-step horizon on the seat's own turn), when it
// waits from every step with priority, the end and cleanup steps
// included; never from the untap step, which grants no priority.
func laterStep(st kop.Step, instant, past bool) bool {
	switch {
	case !instant:
		return st == kop.StepMain1
	case past:
		return st >= kop.StepUpkeep && st <= kop.StepCleanup
	}
	return st >= kop.StepUpkeep && st < kop.StepEnd
}

// passLine is Options.PassMax's value of the root's pass candidate pass
// ("counting pass at the best line after it -- the seat's
// next decision, through the opponent's most-visited replies -- instead of
// its average"): from pass's child node, through every opponent node by
// its most-visited edge, to the seat's next node, whose best child Q
// (the root seat's mean value) among children with at least max(2,
// RootQFrac x its most-visited child's visits) -- the root's own floor --
// is the line's value. ok false (the average stands) when the line ends
// before a seat node or that node has no child over the floor. It reads
// the last tree only (Validate refuses PassMax with CachedWorlds).
func (sr *Searcher) passLine(pass int) (float64, bool) {
	t := &sr.t
	ni := sr.passNext(pass) // instkeys.go
	if ni < 0 {
		return 0, false
	}
	var top int32
	for c := t.nodes[ni].first; c >= 0; c = t.edges[c].sib {
		top = max(top, t.edges[c].n)
	}
	f := sr.Opts.RootQFrac
	if !(f > 0) {
		f = 0.1
	}
	floor := max(2, f*float64(top))
	v, ok := 0.0, false
	for c := t.nodes[ni].first; c >= 0; c = t.edges[c].sib {
		ed := &t.edges[c]
		if float64(ed.n) < floor {
			continue
		}
		if q := ed.w / float64(ed.n); !ok || q > v {
			v, ok = q, true
		}
	}
	return v, ok
}

// reactHoldHorizon is InstWinReact under ReactHorizon (the horizon form of :react):
// an instant-speed cast into an empty stack is held unless the opponent
// did something this turn (the reactTrack tracker), or the timing rules
// offer it no later window before the seat's next untap step (laterStep
// under the untap-step horizon: its last window).
func (sr *Searcher) reactHoldHorizon(s *dirac.State, a dirac.Action, st kop.Step) bool {
	if s.StackLen() != 0 {
		return false
	}
	if !laterStep(st, true, pastTurn(s.ActivePlayer(), s.NextTurnPlayer(), sr.seat, s.OutlastsTurn(a))) {
		return false
	}
	if sr.curReact == nil {
		sr.curReact = &sr.rootReact
	}
	if sr.curReact.firedNow(s, sr.seat) {
		return false
	}
	return !(sr.Opts.ReactAim && sr.aimKeeps(s, a, st)) // instkeys.go
}

// ---- Defer (:defer=w / :noupkeep) -------------------------------------------

// instSpeedCast reports an own-turn cast of an instant or flash card by the
// searching seat that is not a response (no opponent object on the stack).
func (sr *Searcher) instSpeedCast(s *dirac.State, a dirac.Action) bool {
	if a.Kind() != dirac.AkCast || s.ActivePlayer() != sr.seat {
		return false
	}
	def := &s.Table().Cards[s.ObjCard(dirac.Obj(a.Source()))]
	if def.Types&kop.TLand != 0 || (def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0) {
		return false
	}
	for i := 0; i < s.StackLen(); i++ {
		if _, _, _, ctrl := s.StackEntry(i); ctrl != sr.seat {
			return false
		}
	}
	return true
}

// deferBlocked is the :defer= upkeep ban: no own instant-speed spell cast
// in the seat's own upkeep or draw step unless it is a response.
func (sr *Searcher) deferBlocked(s *dirac.State, a dirac.Action) bool {
	st := s.StepNow()
	return (st == kop.StepUpkeep || st == kop.StepDraw) && sr.instSpeedCast(s, a)
}

// deferScale multiplies the prior of each own-main1 instant-speed cast
// candidate by Opts.Defer and renormalises. Defer 1 is a no-op.
func (sr *Searcher) deferScale(s *dirac.State, pt *point) {
	w := sr.Opts.Defer
	if w <= 0 || w == 1 || s.StepNow() != kop.StepMain1 {
		return
	}
	hit := false
	for i := 0; i < pt.n; i++ {
		if pt.off[i+1]-pt.off[i] == 1 && sr.instSpeedCast(s, dirac.Action(pt.acts[pt.off[i]])) {
			pt.prior[i] *= w
			hit = true
		}
	}
	if hit {
		renorm(pt)
	}
}

// ---- PriorTemp (:ptemp=T) ---------------------------------------------------

// temper is Options.PriorTemp at point pt, whose prior holds its
// candidates' logits (before the softmax): each logit divided by T, so the
// softmax is P^(1/T) renormalised. It applies at the searching seat's
// points and, under OpponentMirror, the opponent's; T 0 (off) or 1 leaves
// the logits untouched.
func (sr *Searcher) temper(pt *point, opp bool) {
	t := sr.Opts.PriorTemp
	if t <= 0 || t == 1 || opp && !sr.Opts.OpponentMirror {
		return
	}
	for i := 0; i < pt.n; i++ {
		pt.prior[i] /= t
	}
}

// ---- the root and walk hooks --------------------------------------------------

// timingRootBot is RaidHold at the root's bot answer (Search calls it
// with RaidHold on): pass stands in for a held raid cast, the prior asked
// with the bot's own cast marked (raidMark), Result.RaidBot naming it.
func (sr *Searcher) timingRootBot(d *dirac.Decision, bot int, res *Result) int {
	orig, what := bot, raidNone
	if bot, what = sr.raidBot(&sr.root, d, bot); what != raidNone {
		sr.St.RaidHeld = 1
		if what == raidPass {
			sr.St.RaidHeldBot = 1
			sr.raidMark, res.RaidBot = orig+1, orig+1
		}
	}
	return bot
}

// timingWalk is buildPoint at an in-walk decision under HoldUpkeep or
// RaidHold (advance, sr.tmWalk): the raid substitution before the point,
// the held-cast pass after a failed point, and the walk counters. It
// returns the answer to play when the decision is no point.
func (sr *Searcher) timingWalk(s *dirac.State, d *dirac.Decision, bot int, next *point) (int, bool) {
	raid := raidNone
	if sr.Opts.RaidHold {
		orig := bot
		if bot, raid = sr.raidBot(s, d, bot); raid == raidPass {
			sr.raidMark = orig + 1
		}
	}
	drops := sr.holdDrops
	ok := sr.buildPoint(s, d, bot, next)
	if sr.Opts.HoldUpkeep {
		bot = sr.holdWalk(s, d, bot, ok, drops)
	}
	if raid != raidNone {
		sr.raidMark = 0
		sr.raidWalk(d, raid, ok)
	}
	return bot, ok
}

// timingCands is priorityCands' HoldUpkeep / RaidHold test at decision d
// (bot the bot's answer): whether each window is open, and ok false when
// the bot's answer itself is held (the decision is then unsearched).
func (sr *Searcher) timingCands(s *dirac.State, d *dirac.Decision, bot int) (hold, raid, ok bool) {
	raid = sr.Opts.RaidHold && sr.raidOn(s, d)
	if raid && sr.raidCast(s, d.Options[bot]) {
		return false, false, false
	}
	hold = sr.Opts.HoldUpkeep && sr.holdOn(s, d)
	if hold {
		held := false
		for i := 0; i < int(d.N) && !held; i++ {
			held = sr.holdsCast(s, d.Options[i])
		}
		if held {
			sr.holdDrops++
		}
		if sr.holdsCast(s, d.Options[bot]) {
			return false, false, false // outside the held vocabulary
		}
	}
	return hold, raid, true
}

// timingDrop reports whether option a leaves the candidates as a held cast.
func (sr *Searcher) timingDrop(s *dirac.State, a dirac.Action, hold, raid bool) bool {
	return hold && sr.holdsCast(s, a) || raid && sr.raidCast(s, a)
}

// raidBotMark moves the prior's bot mark from candidate 0 (pass standing
// in) to the bot's own held raid cast (buildPoint, raidMark > 0).
func (sr *Searcher) raidBotMark(d *dirac.Decision) {
	for i := 0; i < int(d.N); i++ {
		sr.botPick[i] = false
	}
	sr.botPick[sr.raidMark-1] = true
}

// cardRootSims is CardSims' budget for the built root point: the largest
// listed entry among its candidates' actions, else sims.
func (sr *Searcher) cardRootSims(sims int) int {
	if n := sr.cardBudget(&sr.root, &sr.rootPt); n > 0 {
		sr.St.CardBoosted = 1
		return n
	}
	return sims
}

// timingRootHeld is HoldUpkeep at an unsearched root: pass in place of a
// held bot cast (every path that plays the bot's answer).
func (sr *Searcher) timingRootHeld(d *dirac.Decision, bot int) int {
	if p := sr.heldAnswer(&sr.root, d, bot); p >= 0 {
		sr.St.HeldUpkeepBot = 1
		return p
	}
	return bot
}

// ---- SkipPass (:skippass[=L]) ----------------------------------------------

// skipWindow is Options.SkipPass at the walk's decision w.d (gorge
// engine/rules skip_pass.go skippableWindow on dirac): a priority
// decision with an empty stack outside a main phase -- level 2: or held by
// the non-active seat -- is answered with its pass, unsearched and without
// the bot, and the walk steps on (true). Never at the root: advance asks
// only after a submit (w.plies > 0). The pass counts as a ply, not a bot
// step (gorge's engine passes it without an intent).
func (sr *Searcher) skipWindow(w *world) bool {
	d := &w.d
	if d.Kind != dirac.DkPriority || w.s.StackLen() != 0 {
		return false
	}
	if st := w.s.StepNow(); (st == kop.StepMain1 || st == kop.StepMain2) && (sr.Opts.SkipPass < 2 || d.Player == w.s.ActivePlayer()) {
		return false
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkPass {
			w.s.Apply(d.Options[i])
			w.plies++
			sr.St.SkipPassed++
			return true
		}
	}
	return false
}

// ---- validation -------------------------------------------------------------

// validateTiming refuses the timing options' bad values (Options.Validate):
// RootQFrac outside (0, 1] or RootLCBZ <= 0 when set, a negative or
// non-finite Defer, PriorTemp, WaitTie or WaitSorc, WaitTie or WaitSorc
// above 1, WaitTieUntap or PassMax without a wait margin, PassMax with
// CachedWorlds (it reads one tree), ReactRoot / ReactHorizon / ReactTarget
// without InstWin == InstWinReact, and the card-name lists'.
func (o *Options) validateTiming() error {
	fin := func(x float64) bool { return x >= 0 && !math.IsInf(x, 1) }
	switch {
	case o.RootQFrac != 0 && !(o.RootQFrac > 0 && o.RootQFrac <= 1):
		return fmt.Errorf("search: RootQFrac %g must be in (0, 1] (0 is 0.1)", o.RootQFrac)
	case o.RootLCBZ != 0 && !(o.RootLCBZ > 0) || math.IsInf(o.RootLCBZ, 1):
		return fmt.Errorf("search: RootLCBZ %g must be > 0 (0 is 1.96)", o.RootLCBZ)
	case !fin(o.Defer):
		return fmt.Errorf("search: Defer %g must be finite and >= 0 (0 is off)", o.Defer)
	case !fin(o.PriorTemp):
		return fmt.Errorf("search: prior temperature %g must be finite and >= 0 (0 is off)", o.PriorTemp)
	case !(o.WaitTie >= 0 && o.WaitTie <= 1) || !(o.WaitSorc >= 0 && o.WaitSorc <= 1):
		return fmt.Errorf("search: WaitTie %g and WaitSorc %g must be in [0, 1]", o.WaitTie, o.WaitSorc)
	case o.WaitSorc != 0 && !o.WaitSorcOn:
		return errors.New("search: WaitSorc needs WaitSorcOn")
	case o.WaitTieUntap && !o.ReadsHorizon(): // instkeys.go: the one list of the horizon's readers
		return errors.New("search: WaitTieUntap needs a wait margin (WaitTie > 0 or WaitSorc > 0), HoldEps, CastPrior, DeferQ or Replan")
	case o.PassMax && !(o.WaitTie > 0 || o.WaitSorcOn && o.WaitSorc > 0):
		return errors.New("search: PassMax needs a wait margin (WaitTie > 0 or WaitSorc > 0)")
	case o.PassMax && o.CachedWorlds > 0:
		return errors.New("search: PassMax reads one tree; it is incompatible with CachedWorlds")
	case o.SkipPass < 0:
		return fmt.Errorf("search: SkipPass %d must be >= 0 (0 is off)", o.SkipPass)
	case (o.ReactRoot || o.ReactHorizon || o.ReactTarget) && o.InstWin != InstWinReact:
		return errors.New("search: ReactRoot, ReactHorizon and ReactTarget need InstWin == InstWinReact")
	}
	return o.validateCardNames()
}
