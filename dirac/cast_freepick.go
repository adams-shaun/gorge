package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// freePickWindowFirst is manaStep's early exit for a cost-waived cast that still
// owes mana (Thorn of Amethyst's {1} on a may-play free cast) and whose
// additional cost picks an object (Sac<N/Spec>, Discard<N/Spec>). gorge poses
// its CR 601.2g window before costObjectAsks (CR 601.2h), so the tax's sources
// are tapped and their mana floats when the pick is asked: a kernel that kept the
// whole payment for bsPay showed the pick an untapped source and an empty pool
// (tail-64 pair 23, Culling the Weak). It taps the way payFreeCast pays (the
// planner's witness when gorge's plan prices the raise, else the window) and
// leaves the spending to bsPay, which finds the cost in the pool. It reports
// whether the cast was reversed because the window ran out of sources (CR 733.1).
func (s *State) freePickWindowFirst(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	if e.flags&sfFree == 0 || *cost == (kop.ManaCost{}) || cost.X != 0 || e.x != 0 ||
		s.cfg&cfgGorgeSurface == 0 || s.rbAny(e.ctrl) {
		return false
	}
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&(kop.CostSac|kop.CostDiscard) == 0 {
		return false
	}
	anyColor := s.bld().bfl&bflAnyColor != 0
	if e.flags&sfMayPlay != 0 && s.bld().bfl&bflLowered == 0 && e.flags&sfAnyMana == 0 && !anyColor &&
		s.castRestPriced(k, e.ctrl, e.obj, cost) {
		s.tapForRider(k, e.ctrl, cost, 0, e.obj, false)
		return false
	}
	ok, held := s.windowTapUseful(k, e.ctrl, cost, cost, e.flags&sfAnyMana == 0 && !anyColor, anyColor, s.bld().pipN == 0, false)
	for c := range held {
		s.pl[e.ctrl].pool[c] += held[c]
	}
	if !ok && !s.winPaused() {
		s.reverseCast(k)
		return true
	}
	return false
}
