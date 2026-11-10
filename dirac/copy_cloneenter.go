package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The as-enters copy election for a clone that enters without being cast
// (a reanimation, a blink). Gorge's applyETBChoiceReplacement parks every
// MoveZone to the battlefield and entryETBChoice asks the ETBReplacement:Copy
// election for the entry, cast or not (CR 614.12, 707.2); the cast ask lives
// in preAskFor. A return that would put such a card onto the battlefield
// during a resolution parks first (preReturn, raGenRetChoice) with the card
// still in its zone; the answer runs the return and becomeCopyEnter consumes
// the pick as the permanent enters.
//
// Every function here is cold and out of line.

// DebugCloneReturnNoAsk drops the election (the mutation test).
var DebugCloneReturnNoAsk bool

// cloneReturnPark poses the clone ask for object o returning to the
// battlefield under ctrl's control; true when the return is parked and the
// caller must suspend without moving the card.
//
//go:noinline
func (s *State) cloneReturnPark(k *prep, o Obj, ctrl uint8) bool {
	if DebugCloneReturnNoAsk {
		return false
	}
	if k.gen == nil || !s.res.active || s.g.pre.state != 0 || s.res.ask != raNone {
		return false
	}
	c := s.objs[o].card
	if !k.gen.has(c, gfEnterRepl) {
		return false
	}
	if s.cloneAskKind(k, c) != 0 {
		s.g.pre = preAsk{obj: o, kind: akClone, state: preReturn}
	} else if kind := s.enterChoiceKind(k, c); kind != 0 {
		s.g.pre = preAsk{obj: o, kind: kind, state: preReturnOp} // Lifecraft Engine's creature type, asked in the graveyard
	} else {
		return false
	}
	s.res.ask, s.res.askP = raGenRetChoice, ctrl
	s.touch(o) // the projection reads ctrl while it waits
	return true
}

// execReturnSimple is the plain OpReturn move of object o (no flags): it
// leaves its zone and goes to in.C, under its owner's control on the
// battlefield.
//
//go:noinline
func (s *State) execReturnSimple(k *prep, in *kop.Instr, o Obj) {
	z, bottom := kop.Zone(in.C), false
	if s.cmdZoneReplace(k, o, &z, &bottom) {
		return // a CR 903.9b commander move parked before it happens
	}
	ob := &s.objs[o]
	from := s.detach(o)
	if k.gen != nil {
		s.g.retMiss = false
	}
	if z == kop.ZoneBattlefield && k.gen != nil && k.gen.anyMayLimit {
		s.countMayPlayLand(k, o, from)
	}
	if z == kop.ZoneBattlefield && k.gen != nil && s.g.batchLatch&batchHeld == 0 && s.enterWithOverride(k, in) {
		// a StaticEffect$ rider (the Enduring cycle's "it's an enchantment")
		// applies before the entry's triggers match the permanent (CR 603.6a,
		// gorge applyStaticEffect ahead of the entry event); a held entry's
		// checks fire after the rider op applies it
		s.enterAssimilated(k, o, ob.owner(), from)
	} else if z == kop.ZoneBattlefield {
		s.enterMoved(k, o, ob.owner(), false, from) // under its owner's control (Kykar's blink of a stolen creature)
	} else {
		s.placeFromK(k, o, z, bottom, from, false)
	}
}

// answerCloneReturn applies the clone pick a to the parked return and
// resumes the program.
//
//go:noinline
func (s *State) answerCloneReturn(k *prep, a Action) {
	s.g.pre.val = cloneAnswer(a)
	if s.cloneReturnOrderPark(k) {
		return // the CR 616.1 order between the copy and another entry replacement (copy_returnorder.go)
	}
	s.finishCloneReturn(k)
}

// finishCloneReturn resumes the parked clone return once its copy election (and
// any CR 616.1 order) is answered. A resolution's return op re-runs the
// instruction (rerunReturn); an until-leaves link return enters the card from
// exile instead, with the answer read by its enters program (CR 614.12, 707.2,
// 610.3).
//
//go:noinline
func (s *State) finishCloneReturn(k *prep) {
	if !s.g.pre.link {
		s.rerunReturn(k)
		return
	}
	o := s.g.pre.obj
	s.g.pre.state = 2
	if s.objs[o].zone == kop.ZoneExile {
		from := s.detach(o)
		s.enterLinkRet(k, o, s.objs[o].owner(), from)
	}
	s.g.pre = preAsk{}
	if s.resumeReturns(k) {
		return // another held return parks its own ask
	}
	s.continueRes(k, 0)
}

// rerunReturn runs the parked return op, whose enters program reads the answer in
// s.g.pre, and resumes the program.
//
//go:noinline
func (s *State) rerunReturn(k *prep) {
	o := s.g.pre.obj
	s.g.pre.state = 2
	in := &s.prog(k)[s.res.pc]
	switch in.Op {
	case kop.OpDigUntil, kop.OpDigEx, kop.OpSearchEx, kop.OpChooseMove, kop.OpChangeZoneAll, kop.OpChangeZoneAllEx:
		s.res.ask = raNone
		if s.rerunMove(k, in) {
			return // suspended again
		}
	case kop.OpReturnEx:
		// the parked card itself, not the op's reference read afresh: the target the
		// card was named by is a bad one now that it stands in exile
		s.execReturnObj(k, in, NoSlot, o)
		s.g.tokDefSet = false // the defender asked before the move (returnDefenderAsk) is spent
		if s.res.ask == raGenEnterDefender {
			// the entry's CR 508.4 defender ask (zone_changezoneparams.go): its answer
			// finishes the instruction, as runProg's suspension does
			s.g.pre = preAsk{}
			s.ctl = ctlResolve
			return
		}
	default:
		s.execReturnSimple(k, in, o)
	}
	s.g.pre = preAsk{}
	if s.eachReturnResume(k) {
		return // the multi-target return's remaining targets (zone_eachreturnask.go)
	}
	s.continueRes(k, 0)
}

// rerunMove runs a found card's move again after its as-enters answer; true when it
// suspended at another ask.
func (s *State) rerunMove(k *prep, in *kop.Instr) bool {
	switch in.Op {
	case kop.OpDigUntil:
		return !s.execDigUntil(k, in, true)
	case kop.OpDigEx:
		return s.finishDig(k)
	case kop.OpChangeZoneAll, kop.OpChangeZoneAllEx:
		return !s.execChangeZoneAll(k, in, true)
	}
	s.finishSearchNow(k)
	return s.res.ask != raNone
}
