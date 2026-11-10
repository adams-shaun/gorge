package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// --- bolster (CR 701.39a) ---

// bolsterCands writes the creatures player p controls with the least
// current (layer-derived) toughness, in battlefield order (gorge
// putCounterBolster: Zone(ZBattlefield, controller), h.Toughness).
func (s *State) bolsterCands(k *prep, p uint8, buf *[MaxPerm]Slot) int {
	var ord [MaxPerm]Slot
	m := s.battlefieldOrder(p&1, s.ctrlM[SeatIx(p)].and(s.occ), ord[:], 0)
	n, best := 0, 0
	for _, sl := range ord[:m] {
		if !s.isCreature(k, sl) {
			continue
		}
		_, t := s.pt(k, sl)
		if n == 0 || t < best {
			best, n = t, 0
		}
		if t == best {
			buf[n] = sl
			n++
		}
	}
	return n
}

// bolster is OpBolster: the resolving controller bolsters A with counters
// of kind C; false: suspended on the tie ask.
func (s *State) bolster(k *prep, in *kop.Instr) bool {
	p := s.res.e.ctrl
	n := s.value(k, in.A)
	if n <= 0 {
		return true
	}
	var buf [MaxPerm]Slot
	switch s.bolsterCands(k, p, &buf) {
	case 0:
		return true
	case 1:
		s.bolsterPut(k, buf[0], int(in.C), n)
		return true
	}
	s.res.ask, s.res.askP = raGenBolster, p
	return false
}

// bolsterPut puts n counters of kind c on the permanent at sl as
// OpPutCounter does (the counter replacements, CounterAddedOnce).
func (s *State) bolsterPut(k *prep, sl Slot, c, n int) {
	s.g.lastCK = kop.CounterKind(c)
	n = s.placeCounters(k, sl, c, n, s.perms[sl].ctrl, ctrPutEffect)
	p := &s.perms[sl]
	if n > 0 && k.anyCtrAdded {
		s.ctrNote(k, sl, c)
	}
	if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
		s.fireObj(k, kop.TrigCounterAddedOnce, sl, p.obj, p.ctrl, int16(n))
	}
}
