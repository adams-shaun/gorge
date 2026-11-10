package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// untapCostOK: the {Q} cost (kop.CostUntapSelf) is payable -- the source
// is tapped and, a creature, not summoning sick without haste (gorge
// manalabel's cost.Untap && !o.Tapped, pay.TapFlagsSick).
//
//go:noinline
func (s *State) untapCostOK(k *prep, sl Slot) bool {
	p := &s.perms[sl]
	if p.flags&pfTapped == 0 {
		return false
	}
	return !s.activationSick(k, sl)
}

// untapCost pays {Q}: the source untaps (gorge cast_commit's "untapped as
// a cost" Untap event, which its Untaps triggers see), a stun counter
// removed instead (CR 122.1d, as OpUntap).
//
//go:noinline
func (s *State) untapCost(k *prep, sl Slot) {
	p := &s.perms[sl]
	s.touchSl(sl)
	if p.flags&pfTapped != 0 && p.ctrLow(int(kop.CtrStun)) > 0 {
		p.seeCtr(s)
		p.subCtrN(s, int(kop.CtrStun), 1)
		return
	}
	if s.untapBlocked(k, sl) {
		return // a replaced Untap event: no Untaps trigger
	}
	p.flags &^= pfTapped
	s.untapEnds(k, p.obj) // the lock and the host-tapped grants end at the first untap (static_animatewhiletapped.go)
	if k.gen != nil && s.subs[kop.TrigUntaps].any() {
		s.fireObjGen(k, kop.TrigUntaps, sl, p.obj, p.ctrl, 0)
	}
}
