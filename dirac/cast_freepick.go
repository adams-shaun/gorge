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

// copyPickWindowFirst is freePickWindowFirst for a Play's paid copy cast
// (CR 707.12, cast_copycast.go): gorge's beginPlay prices no plan for it, so
// the cast is lowered and pays through its CR 601.2g window, which gorge poses
// before costObjectAsks (CR 601.2h). An additional cost that picks an object
// (The Needs of the Many's Sac<1/Creature> under Q, Stacking the Deck's Play)
// is asked with the window's sources already tapped and their mana floating;
// a kernel that kept the whole payment for bsPay asked the pick first (std-trk
// pair 586). It taps the window the way freePickWindowFirst does and leaves the
// spending to bsPay, which reverses a cast the pool still cannot pay after the
// pick (CR 733.1); it never reverses the cast itself, so it reports false.
func (s *State) copyPickWindowFirst(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	if e.flags&sfFree != 0 || *cost == (kop.ManaCost{}) || cost.X != 0 || e.x != 0 ||
		s.cfg&cfgGorgeSurface == 0 || s.rbAny(e.ctrl) || k.gen == nil || !k.gen.anyCopyCast ||
		s.bld().bfl&bflLowered == 0 || s.bld().origin != kop.ZoneLibrary ||
		s.objs[e.obj].of&(ofCopy|ofToken) != ofCopy {
		return false
	}
	pc := s.buildPickCost(k)
	if pc == nil || pc.Flags&(kop.CostSac|kop.CostDiscard) == 0 {
		return false
	}
	anyColor := s.bld().bfl&bflAnyColor != 0
	_, held := s.windowTapUseful(k, e.ctrl, cost, cost, e.flags&sfAnyMana == 0 && !anyColor, anyColor, s.bld().pipN == 0, false)
	for c := range held {
		s.pl[e.ctrl].pool[c] += held[c]
	}
	// a window that runs out of sources does not end the cast here: gorge's
	// manaWindowAsk falls through to costObjectAsks with no source left, asks
	// the pick, and the payment reverses the cast after it (CR 601.2h, 733.1)
	return false
}
