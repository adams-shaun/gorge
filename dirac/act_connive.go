package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// connive is OpConnive (gorge effConnive / conniveOnce, one conniver): a
// conniver no longer on the battlefield connives nothing. Its controller
// draws A, then discards A -- the whole hand when it holds no more, else
// the raDiscard ask (gorge's KModes "discard", Min = Max = A). The nonland
// cards in hand are counted before the discard (genState.conn) for
// OpConniveCounters. It reports false when it suspended on the ask.
//
//go:noinline
func (s *State) connive(k *prep, in *kop.Instr) bool {
	s.g.conn = 0
	if s.objRefObj(in.Obj) == NoObj {
		return true // up to one target, and none chosen
	}
	p := s.lastKnownController(k, in.Obj)
	n := s.value(k, in.A)
	for i := 0; i < n; i++ {
		s.draw(p)
	}
	pl := &s.pl[p]
	s.g.conn = uint8(s.handNonland(k, p)) + 1
	if n <= 0 || pl.handN == 0 {
		return true
	}
	if int(pl.handN) <= n {
		for pl.handN > 0 {
			s.discardObj(k, pl.hand[0])
		}
		return true
	}
	s.res.ask = raDiscard
	s.res.peekI = 0
	s.res.askP = p
	s.res.left = uint8(n)
	s.res.chosen = objWord(p)
	return false
}

// conniveCounters is OpConniveCounters: one +1/+1 counter on the conniver
// per nonland card its controller's hand lost to the connive's discard
// (gorge applyConniveDiscard: one counter event of that many).
//
//go:noinline
func (s *State) conniveCounters(k *prep, in *kop.Instr) {
	before := int(s.g.conn) - 1
	s.g.conn = 0
	sl := s.objRef(k, in.Obj)
	if before < 0 || sl == NoSlot {
		return
	}
	m := before - s.handNonland(k, s.perms[sl].ctrl)
	if m <= 0 {
		return
	}
	m = s.placeCounters(k, sl, int(kop.CtrP1P1), m, s.perms[sl].ctrl, ctrPutEffect)
	if m > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, int(kop.CtrP1P1))
	}
	s.g.lastCK = kop.CtrP1P1
	if m > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(m))
	}
}

// handNonland counts the nonland cards in player p's hand.
func (s *State) handNonland(k *prep, p uint8) int {
	n := 0
	pl := &s.pl[p]
	for i := 0; i < int(pl.handN); i++ {
		if !k.cards[s.objs[pl.hand[i]].card].land {
			n++
		}
	}
	return n
}
