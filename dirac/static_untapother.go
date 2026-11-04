package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugUntapOtherOff is TestUntapoMutation's switch.
var DebugUntapOtherOff bool

// untapOthers is gorge finishUntapStep's foreign scan: the non-active
// player's tapped permanents a StatUntapOther static matches untap, in
// battlefield order (stun: a counter is removed instead), then their
// Untaps triggers fire in that order. None of the active scan's gates
// (exert skip, skip-untap effects, the step-scoped "doesn't untap" gates)
// apply, but an any-event ReplAnyUntap replacement does (untapBlocked).
//
//go:noinline
func (s *State) untapOthers(k *prep) {
	if DebugUntapOtherOff {
		return
	}
	q := 1 - s.active
	var ord [MaxPerm]Slot
	n := s.battlefieldOrder(q, s.ctrlM[q], ord[:], 0)
	var up slotMask
	for _, sl := range ord[:n] {
		p := &s.perms[sl]
		if p.flags&pfTapped == 0 || !s.staticFlagGen(k, kop.StatUntapOther, sl) {
			continue
		}
		if p.ctrLow(int(kop.CtrStun)) > 0 {
			p.seeCtr(s)
			p.subCtrN(s, int(kop.CtrStun), 1) // a stun counter is removed instead (CR 122.1d; turn.go's rule)
			s.touchSl(sl)
			continue
		}
		if s.untapBlocked(k, sl) {
			continue
		}
		p.flags &^= pfTapped
		if s.g.netAny() {
			s.netDrop(p.obj) // the lock ends at the first untap (act_lockpermanent.go)
		}
		s.touchSl(sl)
		up = up.with(sl)
	}
	s.fireUntapsOrdered(k, up, &ord, n)
}
