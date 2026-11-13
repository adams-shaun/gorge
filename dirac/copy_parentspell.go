package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// copyParentSpell is OpCopySpell of the resolving spell itself (Chain
// Lightning; gorge effCopySpellAbility's Defined$ Parent arm): A copies,
// each put on the stack just below the resolving entry, owned by target
// 0's player or its controller with C&4 (gorge copyControllerFor), else the
// resolving controller; C&1 lets the copy's controller choose new targets.
//
//go:noinline
func (s *State) copyParentSpell(k *prep, in *kop.Instr, n int) bool {
	e := &s.res.e
	si := int(s.sp) - 1
	if e.kind != seSpell || si < 0 || s.stack[si].obj != e.obj || s.objs[e.obj].zone != kop.ZoneStack {
		return true
	}
	ctrl := e.ctrl
	if in.C&4 != 0 {
		if p := s.playerRef(k, kop.RefTargetController); p != 255 {
			ctrl = p
		}
	}
	for c := n - 1; c >= 0; c-- {
		if !s.mintSpellCopy(k, si, ctrl) {
			return true
		}
		if in.C&1 != 0 && s.copyElect(k, uint8(c)) { // seat-ok: an instruction flag bit
			return false
		}
		si++ // the original moved up one
	}
	return true
}
