package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Draw replacement body whose last op is a library-look Dig (Tomorrow,
// Azami's Familiar; Underrealm Lich: "look at the top three, put one in your
// hand and the rest ... instead"). The Dig asks its choose (raGenDig, then
// raGenDigArrange for the rest) on the body's own resCtx. At the turn draw the
// kernel keeps that resCtx and the draw step waits on the ask under ctlGenAsk
// as the order ask's site does (repl_draworder.go); the answer runs the
// resolution's ask walk and, when the body's program ends, finishes the draw
// step (drawBodyEnd, drawBodyAnswer). Any other site (a resolving spell's
// OpDraw borrows the spell's resCtx, which cannot carry the body's pc) keeps
// the declared stall: StallDrawRepl, never a silently dropped body.

// isDigAsk reports the pending resolution ask is a Dig's.
func isDigAsk(a uint8) bool {
	return a == raGenDig || a == raGenDigAllOpt || a == raGenDigArrange
}

// drawBodyPark keeps a draw body's Dig ask live at the turn draw (the body
// op at pc i of n, run on a fresh resCtx: saved was inactive) and reports it
// is parked; false: the site cannot wait, the caller stalls.
func (s *State) drawBodyPark(saved *resCtx, i, n int) bool {
	ds := s.drawState()
	if saved.active || i != n-1 || len(ds) == 0 || !ds[0].active || !isDigAsk(s.res.ask) || s.g.nask != 0 {
		return false
	}
	if s.step != kop.StepDraw || s.ctl != ctlStepBegin || !s.g.drawFirst || ds[0].p != s.active {
		return false
	}
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	return true
}

// drawBodyAskGap handles a Draw replacement body's ordinary effect op that
// posed an ask instead of executing (runReplProgAs's default arm, where exec
// returns false): a dredge choice before a body card, or a Dig's choose. The
// body cannot be resumed from there and restoring the outer context wipes the
// ask, so declare the gap rather than drop the body. The one resumable ask is
// a last Dig's, parked at the turn draw. The borrowed resCtx is restored
// except under the park; the caller ends the program either way.
func (s *State) drawBodyAskGap(saved *resCtx, i, n int) {
	if isDigAsk(s.res.ask) {
		if s.drawBodyPark(saved, i, n) {
			return
		}
		s.capFull(StallDrawRepl)
	} else if s.res.ask != raNone {
		s.capFull(StallDredge)
	}
	s.restoreRes(saved, true)
}

// drawBodyAsked: the draw step is suspended on a parked body's Dig ask.
func (s *State) drawBodyAsked() bool {
	if s.ctl != ctlGenAsk || s.res.active || !isDigAsk(s.res.ask) {
		return false
	}
	ds := s.drawState()
	return len(ds) > 0 && ds[0].active
}

// drawBodyEnd is runProg's end for a parked body: its program is done, so
// the in-flight record clears and the borrowed resCtx is dropped instead of
// popping a stack entry. The caller (drawBodyAnswer) finishes the draw step.
func (s *State) drawBodyEnd() bool {
	if s.res.active || s.ctl != ctlGenAsk {
		return false
	}
	ds := s.drawState()
	if len(ds) == 0 || !ds[0].active {
		return false
	}
	s.drawProgramDone()
	s.res = resCtx{}
	return true
}

// drawBodyAnswer applies the parked Dig's answer through the resolution's ask
// walk. While the body asks again (the rest's arrange) the step keeps
// waiting; when it ends, the draw step finishes as drawAskTail does.
func (s *State) drawBodyAnswer(k *prep, a Action) {
	n0 := s.npend
	p := s.drawState()[0].p
	s.applyResolve(k, a)
	switch {
	case s.res.ask != raNone && isDigAsk(s.res.ask) && s.g.nask == 0:
		return // the next ask of the same Dig
	case s.res.ask != raNone || s.g.nask != 0:
		s.capFull(StallDrawRepl) // an ask the draw step cannot wait behind
		return
	}
	if ds := s.drawState(); len(ds) > 0 && ds[0].active {
		s.capFull(StallDrawRepl) // the body did not end
		return
	}
	s.ctl = ctlStepBegin
	s.drawAskTail(k, true, p, 0, 0, 0, n0)
}
