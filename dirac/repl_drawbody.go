package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A Draw replacement body whose last op is a library-look Dig (Tomorrow,
// Azami's Familiar; Underrealm Lich: "look at the top three, put one in your
// hand and the rest ... instead"). The Dig asks its choose (raGenDig, then
// raGenDigArrange for the rest) on the body's own resCtx. At the turn draw the
// kernel keeps that resCtx and the draw step waits on the ask under ctlGenAsk
// as the order ask's site does (repl_draworder.go); the answer runs the
// resolution's ask walk and, when the body's program ends, finishes the draw
// step (drawBodyEnd, drawBodyAnswer).
//
// A resolving spell's OpDraw lends the body its resCtx, which cannot carry
// the body's pc past the ask; the site is resumable through the OpDraw loop
// instead (kw_dredge.go dredgeDrawLoop, the same rails as the body card's
// dredge ask, dredgeBodyPose): the body takes a context of its own (its
// entry is the replacement's ability, so the ask walk reads the body's
// program), the outer resCtx rides the in-flight record, and the loop's
// state rides the record too (drawBodyParkRes). The answer walks the body's
// asks as any resolution dig's does; when the body's program ends,
// drawBodyEndRes restores the outer context and resumes the draw loop past
// the asked card, then the spell's program. A site that is not the loop's
// OpDraw (a connive or Recruit draw, a cascade) keeps the declared stall:
// StallDrawRepl, never a silently dropped body.

// isDigAsk reports the pending resolution ask is a Dig's.
func isDigAsk(a uint8) bool {
	return a == raGenDig || a == raGenDigAllOpt || a == raGenDigArrange
}

// drawBodyPark keeps a draw body's Dig ask live: at the turn draw (the body
// op at pc i of n, run on a fresh resCtx: saved was inactive) the step waits
// under ctlGenAsk; at a resolution site (saved active: the spell's OpDraw)
// the body takes its own context and the outer's rides the record
// (drawBodyParkRes). Reports whether it is parked; false: the site cannot
// wait, the caller stalls.
func (s *State) drawBodyPark(k *prep, saved *resCtx, i, n int, src Slot, abi uint8) bool {
	ds := s.drawState()
	if i != n-1 || len(ds) == 0 || !ds[0].active || !isDigAsk(s.res.ask) || s.g.nask != 0 {
		return false
	}
	if saved.active {
		return s.drawBodyParkRes(k, ds, saved, i, src, abi)
	}
	if s.step != kop.StepDraw || s.ctl != ctlStepBegin || !s.g.drawFirst || ds[0].p != s.active {
		return false
	}
	s.g.ctlRet, s.ctl = ctlStepBegin, ctlGenAsk
	return true
}

// coldDigLoop marks that the draw in flight is inside dredgeDrawLoop's
// resumable loop (kw_dredge.go): the only site where a resolution's Dig body
// ask can be parked (drawBodyParkRes), because the loop lays its state on
// the in-flight record right after dredgeDrawOne returns. A cold region like
// coldDraw, not a fixed-State field (TestNoNewFixedStateFields); only a
// table with a draw program carries the byte.
var coldDigLoop = newCold[uint8]("digLoop", func(g *genPrep) int {
	if g.hasDrawProgram() {
		return 1
	}
	return 0
})

// digLoopMark sets the coldDigLoop byte (a table without the region keeps
// none).
func (s *State) digLoopMark(v bool) {
	if r := coldDigLoop.of(s); len(r) > 0 {
		if v {
			r[0] = 1
		} else {
			r[0] = 0
		}
	}
}

// digLoopMarked reports the draw in flight is dredgeDrawLoop's.
func (s *State) digLoopMarked() bool {
	r := coldDigLoop.of(s)
	return len(r) > 0 && r[0] == 1
}

// drawBodyParkRes parks a resolution's Dig body ask (the op at pc i of the
// body's program): the site must be dredgeDrawLoop's OpDraw (the marker, and
// the outer context's op, a plain ability program's OpDraw: no cascade or
// overload frame to restore under), and no other ask may be queued. The body
// already runs on its own entry (runReplProgAs), so the ask builders and
// answers read the body's program exactly as at the turn draw; the outer
// resCtx and the loop's state ride the record for drawBodyEndRes.
func (s *State) drawBodyParkRes(k *prep, ds []drawProgram, saved *resCtx, i int, src Slot, abi uint8) bool {
	if !s.digLoopMarked() || s.g.nask != 0 {
		return false
	}
	if saved.e.card == NoCard || saved.e.ab >= abCascade ||
		int(saved.e.ab) >= len(k.t.Cards[saved.e.card].Abilities) {
		return false
	}
	outer := k.t.Cards[saved.e.card].Abilities[saved.e.ab].Prog
	if int(saved.pc) >= len(outer) || outer[saved.pc].Op != kop.OpDraw {
		return false
	}
	if q := s.prog(k); len(q) == 0 || &q[0] != &k.t.Cards[s.cardAt(src)].Abilities[abi].Prog[0] {
		return false // an overload or special frame claims s.prog: the ask walk would read the wrong op
	}
	d := &ds[0]
	d.outer = *saved
	d.outerOn = true
	d.bodyE = s.res.e
	s.res.pc = uint8(i)
	return true
}

// drawBodyResAsked reports the pending decision is a Draw replacement body's
// Dig ask parked at a resolution site: the in-flight record carries a parked
// outer context and the live context is the body's own entry.
func (s *State) drawBodyResAsked() bool {
	ds := s.drawState()
	return len(ds) > 0 && ds[0].active && ds[0].outerOn && isDigAsk(s.res.ask) && s.res.e == ds[0].bodyE
}

// drawBodyEndRes is runProg's end for a body parked at a resolution site:
// its program is done, so the outer resolution's context returns, the OpDraw
// loop goes on past the asked card, and the spell's program follows. True:
// the ending program was a parked body's.
func (s *State) drawBodyEndRes(k *prep) bool {
	ds := s.drawState()
	if len(ds) == 0 {
		return false
	}
	d := ds[0]
	if !d.active || !d.outerOn || s.res.e != d.bodyE {
		return false
	}
	s.drawProgramDone()
	s.res = d.outer
	in := &s.prog(k)[s.res.pc]
	if s.dredgeDrawLoop(k, in, d.p, int(d.outer.left)-1, d.rest, int(d.pickN)) {
		s.continueRes(k, 0)
	}
	return true
}

// drawBodyAskGap handles a Draw replacement body's ordinary effect op that
// posed an ask instead of executing (runReplProgAs's default arm, where exec
// returns false): a dredge choice before a body card, or a Dig's choose. The
// body cannot be resumed from there and restoring the outer context wipes the
// ask, so declare the gap rather than drop the body. The one resumable ask is
// a last Dig's, parked at the turn draw or at the spell's resumable OpDraw
// loop (drawBodyPark). The borrowed resCtx is restored except under the park;
// the caller ends the program either way.
func (s *State) drawBodyAskGap(k *prep, saved *resCtx, i, n int, src Slot, abi uint8) {
	if isDigAsk(s.res.ask) {
		if s.drawBodyPark(k, saved, i, n, src, abi) {
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
