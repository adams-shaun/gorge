package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// sacPartReserved is the set of p's mana sources a spell's Sac<N/Spec> additional
// cost still needs to sacrifice: the candidates that are free of mana (a plain
// artifact) cover the part first, and only the remainder is reserved from the
// sources that sacrifice themselves for mana (a Treasure), in slot order. gorge
// prices the cast's mana with those candidates held back (planReserving; CR
// 601.2g-h), so the largest-X witness never taps the Treasure the part
// sacrifices (Reshape with one Treasure: X = 0, not 1).
func (s *State) sacPartReserved(k *prep, p uint8, part *kop.Cost) slotMask {
	if part.Flags&kop.CostSac == 0 || !k.sacMana {
		return slotMask{}
	}
	var cons slotMask
	free := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		x := m.first()
		if !s.matchPerm(k, part.Filter, x, p, NoSlot) {
			continue
		}
		pc := s.manaPCAt(k, x)
		if pc.manaSac && s.canTapForManaPC(k, x, pc) || pc.manaSacOnly && s.canSacOnlyNow(k, x, pc) {
			cons = cons.or(slotBit(x))
			continue
		}
		free++
	}
	need := int(part.N) - free
	var held slotMask
	for ; need > 0 && cons.any(); need-- {
		held = held.or(slotBit(cons.first()))
		cons = cons.rest()
	}
	return held
}

// xSacWindowFirst is manaStep's early exit for a cast whose payment waits for
// bsPay, with one exception: an announced {X} paid through the CR 601.2g window
// taps before the sacrifice is chosen (gorge manaWindowAsk before costObjectAsks),
// and when the window takes the mana source the cost's Sac part needed (X = 1
// beside the pool's {U}{U} spends Reshape's only artifact, a Treasure), the
// part has no candidate left and the cast reverses (CR 601.2h, 733.1; the
// tapped mana stays floating). It reports whether the cast was reversed.
func (s *State) xSacWindowFirst(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	if e.flags&sfFree != 0 || cost.X == 0 || e.x == 0 || s.cfg&cfgGorgeSurface == 0 ||
		s.bld().bfl&bflLowered == 0 || !s.sacPartReserved(k, e.ctrl, spellAddCost(k, e.card)).any() {
		return false
	}
	if !s.xWindowPay(k, e, cost) {
		s.reverseCast(k)
		return true
	}
	return false
}
