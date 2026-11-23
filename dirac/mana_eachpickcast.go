package dirac

// eachPickSources is the slots of p's all-at-once mana sources that a reflect
// carrier's pick widens (srcEachPick: Jungle Basin's {C}{G} under Mirari's
// Wake). gorge's planner names no one type over the several-type production
// (rules paymentPlanTapObservers: PaymentPlanReflectedMana folds one type
// only), so its census defers the source (source:interference) and a cast only
// that source pays is scripted, never planned: on the autopay surface it is
// not on gorge's priority list (FeatPlannedCast), though the kernel's own
// census prices the pick as a unit (addEachPick; tail-15 pair 29, Bamboo Grove
// Archer beside Jungle Basin and a Lifespring Druid).
func (s *State) eachPickSources(k *prep, p uint8) (m slotMask) {
	if k.gen == nil || !k.gen.anyTapExtra {
		return m
	}
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.srcEachPick(k, sl, s.manaPCGen(k, sl)) > 0 {
			m = m.with(sl)
		}
	}
	return m
}
