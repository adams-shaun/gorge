package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// DrawOrderStats counts the lockstep test's coverage: asks posed.
var DrawOrderStats struct{ Asked atomic.Int64 }

// The CR 616.1 order ask of competing Draw replacements (gorge
// poseReplacementChoice on a Draw event). Two replacements matching one draw
// do not commute when one of them is a ReplDrawProgram body, so the drawing
// player picks which applies; the picked candidate replaces the draw
// (CR 616.1e: the draw event is gone, the body's own draws meet the others
// through the draw's lineage, repl_draw.go drawChain), exactly as the single
// candidate's walk does (drawApply). Candidates that all commute (plain
// ReplDrawMore) settle in scan order unasked.
//
// The ask is posed where the draw can be suspended:
//   - an OpDraw card: the resolution pauses on raGenDrawOrder with the loop's
//     state laid as the dredge ask lays it (res.left, g.pickZ, g.pickLeft;
//     kw_dredge.go dredgeDrawLoop), and the answer applies the pick and
//     draws on;
//   - the turn draw (CR 504.1): the draw step suspends on ctlGenAsk, and the
//     answer finishes the step as dredgeTurnAnswer does.
//
// A draw at any other instruction (an effect that draws without OpDraw: a
// connive, a cost), a lineage already holding a source, more than one plain
// ReplDrawMore beside the program, or more than drawCandMax candidates keeps
// the declared StallDrawRepl. A trigger's OpDraw is a resolution's OpDraw and
// is resumable like any other, unless it carries RememberDrawn$: its hand poll
// runs when the draw returns (zone_rememberwrites.go drawRemember), before a
// suspended body could draw, so that shape keeps the stall too.

// drawOrderOn: the table can pose the ask, so OpDraw runs its resumable loop.
func drawOrderOn(k *prep) bool { return k.gen != nil && k.gen.hasDrawProgram() }

// drawOrderSite reports the site the draw p is about to make is resumable.
func (s *State) drawOrderSite(k *prep, p uint8) (op, turn bool) {
	if s.res.active {
		pr := s.prog(k)
		return int(s.res.pc) < len(pr) && pr[s.res.pc].Op == kop.OpDraw && pr[s.res.pc].C&kop.MvRemember == 0 && s.res.ask == raNone, false
	}
	return false, s.step == kop.StepDraw && s.ctl == ctlStepBegin && s.g.drawFirst && p == s.active
}

// drawOrderPose poses the order ask over the candidates c of the draw p is
// about to make; true: the draw is taken (asked, or stalled).
func (s *State) drawOrderPose(k *prep, p uint8, ch drawChain, c *drawCands) bool {
	op, turn := s.drawOrderSite(k, p)
	if ch.n != 0 || c.all > drawCandMax || c.more > 1 || !op && !turn {
		s.capFull(StallDrawRepl)
		return true
	}
	s.res.ask, s.res.askP = raGenDrawOrder, p
	DrawOrderStats.Asked.Add(1)
	if turn {
		s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	}
	return true
}

// drawOrderOptions poses the ask: one option per candidate, its source and
// scan index (gorge's KReplacement order ask).
//
//kmanifest:askopts raGenDrawOrder
func (s *State) drawOrderOptions(k *prep, d *Decision) {
	p := s.res.askP
	c := s.drawScan(k, p, drawChain{})
	d.Kind, d.Sub, d.Player = DkChoose, ChGenReplOrder, p
	for i := 0; i < c.n; i++ {
		d.add(MakeAction(AkChoose, TCode(s.perms[c.sl[i]].obj), 0, uint8(i)))
	}
}

// drawOrderAnswer applies the pick to the draw and carries the site on: the
// OpDraw loop draws the rest of its cards, the turn draw finishes the step.
//
//kmanifest:askapply raGenDrawOrder
func (s *State) drawOrderAnswer(k *prep, a Action) {
	p := s.res.askP
	c := s.drawScan(k, p, drawChain{})
	turn := !s.res.active
	s.res.ask = raNone
	if turn {
		s.ctl = ctlStepBegin
	}
	left, rest, n := int(s.res.left), s.g.pickZ, int(s.g.pickLeft)
	n0 := s.npend
	i := int(a.Mode())
	if i >= c.n {
		s.capFull(StallDrawRepl) // the candidates changed under the ask
		return
	}
	s.drawApply(k, p, c.sl[i], c.ab[i], drawChain{})
	if s.res.ask == raGenDrawMay {
		// the pick is an Optional$ True line: gorge promotes it to its own
		// apply / decline (scheduleSelectedOptionalDraw), whose answer runs
		// this site's tail (repl_drawmay.go)
		return
	}
	if s.res.ask != raNone {
		s.capFull(StallDrawRepl) // the body posed an ask neither site can wait behind
		return
	}
	s.drawAskTail(k, turn, p, left, rest, n, n0)
}
