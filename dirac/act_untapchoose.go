package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// ChGenUntapPick is OpUntapChoose's pick (gorge's "untap" KChoose):
// source = the permanent untapped; AkDone ends an up-to pick.
const ChGenUntapPick ChoiceKind = 201

// untapChoose is OpUntapChoose (gorge effUntap's UntapType$ arm; Snap's
// "untap up to two lands"): the candidates are every battlefield
// permanent matching F, seat 0's first, each in battlefield order, tapped
// or not; n is capped at their number. An up-to pick, or more candidates
// than n, is asked one permanent at a time (Done ends an up-to pick);
// otherwise every candidate untaps. The picks untap together afterwards,
// in pick order (TryUntap: a stun counter is removed instead).
func (s *State) untapChoose(k *prep, in *kop.Instr) bool {
	var buf [MaxPerm]Slot
	s.res.peekN = 0
	c := s.untapChooseCands(k, in, &buf)
	n := s.value(k, in.A)
	if n <= 0 || c == 0 {
		return true
	}
	n = min(n, c)
	if in.C != 1 && c <= n {
		for _, sl := range buf[:c] {
			s.untapSlot(k, sl)
		}
		return true
	}
	s.res.ask, s.res.askP, s.res.left = raGenUntapPick, s.res.e.ctrl, uint8(n)
	return false
}

// untapChooseCands writes the not-yet-picked permanents matching the op's
// filter, seat 0 first, each seat in battlefield order.
func (s *State) untapChooseCands(k *prep, in *kop.Instr, buf *[MaxPerm]Slot) int {
	var m slotMask
	src := s.resSrcSlot(k)
	for q := s.occ; q.any(); q = q.rest() {
		sl := q.first()
		if !s.matchPerm(k, in.F, sl, s.res.e.ctrl, src) {
			continue
		}
		picked := false
		for i := 0; i < int(s.res.peekN); i++ {
			picked = picked || s.res.peek[i] == s.perms[sl].obj
		}
		if !picked {
			m = m.with(sl)
		}
	}
	return s.listenerOrder(m, buf) // every seat's battlefield, seat by seat
}

// untapSlot is the core's OpUntap on the permanent at sl (gorge TryUntap).
func (s *State) untapSlot(k *prep, sl Slot) {
	p := &s.perms[sl]
	if p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
		p.seeCtr(s)
		p.subCtrN(s, int(kop.CtrStun), 1)
		s.touchSl(sl)
		return
	}
	if s.untapBlocked(k, sl) {
		return
	}
	s.untapEffect(k, sl)
	s.touchSl(sl)
}
