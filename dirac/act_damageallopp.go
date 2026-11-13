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
	var pl uint8 // dmgSeqPl's bits: 1 the active player, 2 the other; the seat mask at more seats
	if in.B.N&2 != 0 && MaxSeats != 2 {
		pl = seatBits(s.oppsFrom(ctrl))
	} else if in.B.N&2 != 0 {
		if NextSeat(ctrl) == s.active {
			pl = 1
		} else {
			pl = 2
		}
	}
	if k.gen != nil && (k.gen.anyRepl&(rbDblDmg|rbPrevNC|rbPrevDmgTo|rbPrevCol) != 0 || shieldBesidePlus(k.gen)) {
		s.damageSeqStart(k, hit, false)
		s.g.dmgSeqPl = pl
		s.damageSeqNext(k, n)
		return
	}
	// Same sweep order as OpDamageAll's plain path (gorge effDamageAll:
	// each seat's battlefield in entry order, seat 0 first, then each
	// opponent from the controller, players last).
	var ord [MaxPerm]Slot
	hi := 0
	for p := uint8(0); p < s.nSeats(); p++ {
		hi = s.battlefieldOrder(p, hit.and(s.ctrlM[p]), ord[:], hi)
	}
	for _, sl := range ord[:hi] {
		s.damageTo(k, uint8(s.perms[sl].obj), n)
	}
	if pl != 0 {
		q := s.oppsFrom(ctrl)
		for i := uint8(0); i < q.n; i++ {
			s.damageTo(k, uint8(PlayerCode(q.s[i])), n)
		}
	}
}
