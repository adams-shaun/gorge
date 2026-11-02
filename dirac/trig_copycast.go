package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fireCopyTrigs fires the SpellCast lines that also see spell copies
// (kop.Tf2OrCopy, gorge Mode$ SpellCastOrCopy matching the StackCopy
// event) for the copy e just put on the stack, in listener order, as the
// cast path matches a spell (castTrigGen).
//
//go:noinline
func (s *State) fireCopyTrigs(k *prep, e *stackEntry) {
	lm := s.subs[kop.TrigSpellCast].and(s.occ)
	if k.gen.grantTrigEv&(1<<kop.TrigSpellCast) != 0 {
		lm = s.grantTrigHosts(k, lm)
	}
	if !lm.any() {
		return
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(lm, &ord)
	orCopy := func(a *kop.Ability) bool {
		return a.Trig.Event == kop.TrigSpellCast && a.Trig.Flags2&kop.Tf2OrCopy != 0
	}
	for _, sl := range ord[:n] {
		s.slotTriggers(k, sl, orCopy, func(l trigListener, a *kop.Ability) {
			s.castTrigGen(k, l, a, e)
		})
	}
}
