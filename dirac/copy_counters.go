package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtCopyTokenCtr: CopyPermanent WithCountersType$/WithCountersAmount$
// (gorge effects/copypermanent.go folds the counters into the copy's own
// entry MoveZone through entryWithCounters). The counters are part of each
// token's entry (CR 122.6, 614.1c): s.g.enterCtr is set immediately before
// the copy enters, so enterGen puts them through the counter replacements
// with the entry, not as a later put. Counters are not copiable values
// (CR 707.2): the source's own are never read here.

// copyTokenCtr is ExtCopyTokenCtr. A copy with an as-enters choice (a
// colour, a type: Littjara Mirrorlake on Chameleon Spirit) waits outside
// every zone for it, as copyTokensRun's does, and enters with the counters
// when it is answered (copyTokenCtrAnswer). An Aura or a clone election
// would have to ask while the rider waits on another path, so those stall
// rather than drop the counters; so does a choice when the amount is not a
// constant (it is read once, and the answer resumes after a re-read).
// True when a copy parked on its ask.
//
//go:noinline
func (s *State) copyTokenCtr(k *prep, in *kop.Instr) bool {
	e := &s.res.e
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		return false
	}
	c := s.objs[o].card
	if s.objs[o].zone == kop.ZoneBattlefield {
		c = s.cardAt(s.objs[o].slot)
	}
	amt := s.value(k, in.B) // once, before any token (gorge resolves it once per call)
	cin := copyTokenCtrInstr(in)
	tc, n, rewritten, ok := s.copyTokenRewrite(k, e.ctrl, c, 1, false)
	if !ok {
		return false
	}
	if rewritten {
		// a CreateToken rewriter replaced the copy with a plain mint of its
		// card: no copy is left for the counters (repl_tokrewrite.go)
		s.mintRewritten(k, e.ctrl, tc, n)
		return false
	}
	if n > 0 && (k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject ||
		s.enterChoiceKind(k, c) != 0 && in.B.K != kop.VConst ||
		k.gen != nil && k.gen.anyBecomeCopy && s.cloneTokenAsk(k, &cin, c) != 0) {
		s.stall(StallCopyAllAsk)
		return false
	}
	return s.copyTokenCtrMint(k, in, c, n, amt)
}

// copyTokenCtrInstr is the clean copy instruction the rider's tokens enter
// through: the copy path reads B as CopyFlags and C as the copy-exception
// override, neither of which the rider carries.
func copyTokenCtrInstr(in *kop.Instr) kop.Instr {
	return kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: in.Obj, A: kop.Const(1)}
}

// copyTokenCtrMint mints n copies of card c, each entering with amt
// counters of in's kind. A copy with an as-enters choice parks on it
// (raGenTokChoice; genState.pickLeft the copies after it) and true is
// returned.
func (s *State) copyTokenCtrMint(k *prep, in *kop.Instr, c CardID, n, amt int) bool {
	e := &s.res.e
	cin := copyTokenCtrInstr(in)
	choice := s.enterChoiceKind(k, c)
	for i := 0; i < n; i++ {
		if s.nearCapacity(k) {
			s.stall(StallBoard)
			return false
		}
		if choice != 0 {
			t := s.parkCopy(c, e.ctrl, raGenTokChoice)
			s.g.pickLeft = uint8(n - i - 1)
			s.g.pre = preAsk{obj: t, kind: choice}
			return true
		}
		t := s.newToken(c, e.ctrl)
		s.objs[t].of |= ofCopy
		s.copyTokenCtrEnter(k, &cin, t, in, amt)
	}
	return false
}

// copyTokenCtrEnter enters copy token t with amt counters of in's kind as
// part of its entry.
func (s *State) copyTokenCtrEnter(k *prep, cin *kop.Instr, t Obj, in *kop.Instr, amt int) {
	if amt > 0 {
		s.g.enterCtr.n, s.g.enterCtr.kind = uint8(min(amt, 255)), kop.CounterKind(in.C)
	}
	s.copyTokenEnter(k, cin, t, NoSlot, nil)
	s.g.enterCtr.n = 0
}

// copyTokenCtrAnswer is the raGenTokChoice answer for an ExtCopyTokenCtr
// copy: the answered copy t enters with its counters (the enters program
// reads the choice), then the copies after it mint. The amount is a
// constant here (copyTokenCtr stalls otherwise). True when a later copy
// parked again.
func (s *State) copyTokenCtrAnswer(k *prep, in *kop.Instr, t Obj) bool {
	amt := s.value(k, in.B)
	cin := copyTokenCtrInstr(in)
	s.copyTokenCtrEnter(k, &cin, t, in, amt)
	s.g.pre = preAsk{}
	s.res.ask = raNone
	return s.copyTokenCtrMint(k, in, s.objs[t].card, int(s.g.pickLeft), amt)
}

// tokChoiceResume is the raGenTokChoice answer's resume once the answered
// copy t is out of pending and the resolution's controller is the copy's
// (was: the one before): t enters with the choice and the copies after it
// mint. True when a later copy parked again or the op stalled.
func (s *State) tokChoiceResume(k *prep, in *kop.Instr, t Obj, was uint8) bool {
	if in.Op == kop.OpExt && kop.ExtOp(in.A.N) == kop.ExtCopyTokenCtr {
		return s.copyTokenCtrAnswer(k, in, t) // the rider's counters enter with it
	}
	if s.g.sweep.on { // copy_allpark.go: a copy-all sweep's parked copy
		return s.sweepAnswer(k, in, t, s.g.sweep.host) // the enters program reads the choice
	}
	s.copyResumedEnter(k, in, t, NoSlot) // the enters program reads the choice
	s.g.pre = preAsk{}
	s.res.ask = raNone
	if s.copyTokens(k, in, s.objs[t].card, int(s.g.pickLeft)) {
		return true
	}
	if s.eachResume(k, in) { // copy_eachtarget.go: the targets after it
		return true
	}
	return s.nonRemResume(k, in, was, s.res.e.ctrl) // copy_nonremctrl.go: the owners after it
}
