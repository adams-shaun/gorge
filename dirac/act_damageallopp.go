package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// damageAllOpp is OpDamageAll with its B.N bits 2 (each opponent of the
// controller is dealt the damage too, after the permanents) and 4 (the
// filter's own types choose the permanents: "each creature and
// planeswalker they control"), End the Festivities (gorge effDamageAll:
// ValidCards$ over both battlefields, then ValidPlayers$, one batch).
//
//go:noinline
func (s *State) damageAllOpp(k *prep, in *kop.Instr, ctrl uint8, n int) {
	src := s.objRef(k, kop.RefSelf)
	var hit slotMask
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if (in.B.N&4 != 0 || s.isCreature(k, sl)) && s.matchPerm(k, in.F, sl, ctrl, src) {
			hit = hit.with(sl)
		}
	}
	var pl uint8 // dmgSeqPl's bits: 1 the active player, 2 the other
	if in.B.N&2 != 0 {
		if 1-ctrl == s.active {
			pl = 1
		} else {
			pl = 2
		}
	}
	if k.gen != nil && k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevCol) != 0 {
		s.damageSeqStart(k, hit, false)
		s.g.dmgSeqPl = pl
		s.damageSeqNext(k, n)
		return
	}
	for m := hit; m.any(); m = m.rest() {
		s.damageTo(k, uint8(s.perms[m.first()].obj), n)
	}
	if pl != 0 {
		s.damageTo(k, PlayerTarget0+1-ctrl, n)
	}
}
