package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A DrawProgram body's hidden pick. Out of the Tombs' ReplaceWith$ body is a
// mandatory hidden ChangeZone from a graveyard onto the battlefield; the
// generic OpChooseMove ask (raGenSearch) cannot be resumed from inside a
// replacement program because its answer continues the resolution the
// program borrowed. The body therefore parks the pick as a queued ask, the
// same mechanism the as-enters reveal uses (repl_enterreveal.go): the ask
// record carries the program's card/ability/pc, so answerQueuedAsk's
// runReplProg resumes the body after the chosen card has moved.
//
// Mid-resolution the parked pick also suspends its OpDraw site
// (dredgeDrawLoop's drawPickAsked arm, kw_dredge.go): gorge poses the pick
// with the resolving entry still on the stack and every remaining card of
// the same draw still owed (engine/rules/repl_draw_empty.go replaces each
// draw event on its own), so the loop waits on the ask tape and the answer
// carries it on through the rest of the Draw (drawPickResume).
const akDrawPick uint8 = 38

// DrawPickStats counts the draw-repl pool's branch coverage for the lockstep
// test (oracle/repl_drawrepl_lockstep_test.go): how often a DrawProgram
// body's hidden pick moved at least one card, and how often its
// Remembered-gated LosesGame ran while the body was in flight.
// DrawOrderStats (repl_draworder.go) is the precedent.
var DrawPickStats struct{ Moved, Lost atomic.Int64 }

// drawPickStart is runReplProgAs' DrawProgram OpChooseMove arm: it runs the
// pick's setup (startSearch) and, when a choice is to be posed, converts the
// posed raGenSearch into the queued akDrawPick. It reports true when the
// program must wait for the answer.
func (s *State) drawPickStart(k *prep, sl Slot, c CardID, ab, pc uint8, in *kop.Instr) bool {
	if in.Op != kop.OpChooseMove {
		return false
	}
	// startSearch reads the resolving program through s.prog/s.res.pc. A
	// DrawProgram inside an outer resolution borrows that outer resCtx, so
	// install the replacement's own entry around the setup, then put the
	// outer entry back (the posed ask and pick state stay in s.res).
	saveE, savePC := s.res.e, s.res.pc
	s.res.e.kind = seAbility
	s.res.e.card = c
	s.res.e.ab = uint8(ab)
	s.res.e.ctrl = s.perms[sl].ctrl
	s.res.e.obj = s.perms[sl].obj
	s.res.e.srcVer = s.objs[s.perms[sl].obj].ver
	s.res.pc = pc
	ok, done := s.execGen(k, in)
	s.res.e, s.res.pc = saveE, savePC
	if !done || ok {
		return false // the pick completed in place (or the op was not a pick)
	}
	if s.res.ask == raGenSearch && !s.res.active {
		// gorge serves a mid-resolution ask only inside a resolution run;
		// a draw step's own draw (or any draw outside a resolution) takes
		// the no-host fallback instead: the first eligible cards in zone
		// order. Match it here so the kernel does not pose a decision gorge
		// never asks.
		s.drawPickFallback(k)
		return false
	}
	if s.res.ask != raGenSearch {
		// Not the hidden pick shape this path supports (an Optional$ confirm
		// would need its own queued answer). The kgen shape guard admits only
		// the mandatory hidden pick, so this is a table/kernel mismatch.
		s.capFull(StallAsk)
		return true
	}
	s.res.ask = raNone
	s.queueAskAt(sl, akDrawPick, c, ab, pc+1)
	return true
}

// drawPickFallback completes a DrawProgram hidden pick the way gorge's
// no-host path does: the first eligible cards in zone order, moved with the
// pick's own destination and the RememberChanged rider.
func (s *State) drawPickFallback(k *prep) {
	var d Decision
	s.searchCands(k, &d)
	n := int(s.g.pickLeft)
	if dn := int(d.N); n > dn {
		n = dn
	}
	for i := 0; i < n; i++ {
		s.resArm().look[s.g.nlook] = Obj(d.Options[i].Source())
		s.g.nlook++
		s.g.pickLeft--
	}
	if n > 0 {
		DrawPickStats.Moved.Add(1)
	}
	s.res.ask = raNone
	s.finishSearchNow(k)
}

// drawPickIn is the OpChooseMove a queued DrawProgram pick is answering.
func (s *State) drawPickIn(k *prep, a *askRec) *kop.Instr {
	return &k.t.Cards[a.card].Abilities[a.ab].Prog[a.pc-1]
}

// withDrawPickCtx runs fn with the DrawProgram body the queued pick belongs to
// current: searchCands and finishSearchNow read the resolving program through
// s.prog/s.res.pc.
func (s *State) withDrawPickCtx(k *prep, a *askRec, fn func()) {
	save := s.res
	s.res.pc = a.pc - 1
	s.res.e.card = a.card
	s.res.e.ab = uint8(a.ab)
	s.res.e.ctrl = s.perms[a.sl].ctrl
	s.res.e.obj = s.perms[a.sl].obj
	fn()
	s.res = save
}

// drawPickOptions builds the hidden pick's offered cards.
func (s *State) drawPickOptions(k *prep, d *Decision) {
	a := &s.g.ask[0]
	in := s.drawPickIn(k, a)
	d.Sub = ChGenHiddenPick
	s.withDrawPickCtx(k, a, func() { s.searchCands(k, d) })
	if in.C>>3&kop.SrMandatory == 0 {
		d.add(MakeAction(AkDone, 0, 0, 0))
	}
}

// drawPickAnswer applies one answer; false keeps the pick queued.
func (s *State) drawPickAnswer(k *prep, ask askRec, a Action) bool {
	if a.Kind() == AkDone {
		return true // the mandatory shape offers no Done; a Done ends the pick
	}
	o := Obj(a.Source())
	s.resArm().look[s.g.nlook] = o
	s.g.nlook++
	s.g.pickLeft--
	if s.g.pickLeft > 0 {
		s.requeueDrawPick(ask)
		return false
	}
	if s.g.nlook > 0 {
		DrawPickStats.Moved.Add(1)
	}
	s.withDrawPickCtx(k, &ask, func() { s.finishSearchNow(k) })
	return true
}

// drawPickAsked reports that the draw replacement program just run parked its
// hidden pick as a queued ask: drawProgramTake left the record live (a body
// that finishes clears it) and the pick's record is in the ask queue. The
// OpDraw site suspends on it (kw_dredge.go dredgeDrawLoop).
func (s *State) drawPickAsked() bool {
	ds := s.drawState()
	if len(ds) == 0 || !ds[0].active {
		return false
	}
	for i := 0; i < int(s.g.nask); i++ {
		if s.g.ask[i].kind == akDrawPick {
			return true
		}
	}
	return false
}

// drawPickWaiting reports the pick's OpDraw frame laid down by that suspend:
// a pick whose body is still open (the answer re-queued a multi-card pick)
// or whose body finished while drawPickResume had not yet carried the draw
// on. The suspended draw instruction stays put behind it: an older queued
// ask answered meanwhile must not continueRes past the draw.
func (s *State) drawPickWaiting() bool {
	ds := s.drawState()
	return len(ds) > 0 && ds[0].waiting
}

// drawPickResume carries the OpDraw site suspended at drawPickAsked on once
// the pick's answer has finished the body: the in-flight record dies first
// (each later card of the same draw is its own draw event and meets the
// replacement again, CR 614.5), the loop draws the cards it still owed, and
// the program continues past the draw instruction. False: the pick's body is
// not finished (the multi-card pick re-queued); the answer's next ask pose
// serves it.
func (s *State) drawPickResume(k *prep) bool {
	ds := s.drawState()
	if len(ds) == 0 || !ds[0].waiting || ds[0].active {
		return false // no frame, or the body still open on another answer
	}
	p, left, rest, n := ds[0].p, int(ds[0].left)-1, ds[0].rest, int(ds[0].n)
	ds[0] = drawProgram{}
	if s.dredgeDrawLoop(k, &s.prog(k)[s.res.pc], p, left, rest, n) {
		s.continueRes(k, 0)
	}
	return true
}

// noteDrawReplLoss records a LoseGame that ran while a DrawProgram body was
// in flight: Out of the Tombs' Remembered-gated branch, which fires exactly
// when the pick moved nothing (interp.go's OpLoseGame arm).
func (s *State) noteDrawReplLoss() {
	if ds := s.drawState(); len(ds) > 0 && ds[0].active {
		DrawPickStats.Lost.Add(1)
	}
}

// requeueDrawPick puts the popped pick back at the queue's head for the next
// answer; a full queue is a declared stall.
func (s *State) requeueDrawPick(ask askRec) {
	if int(s.g.nask) >= len(s.g.ask) {
		s.capFull(StallAsk)
		return
	}
	copy(s.g.ask[1:s.g.nask+1], s.g.ask[:s.g.nask])
	s.g.nask++
	s.g.ask[0] = ask
}
