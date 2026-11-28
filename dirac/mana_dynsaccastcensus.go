package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// affordsUngrantedCast is affordsUngrantedFor for the spell cast (o, its own
// card) gorge's planner would have to price: a dynamic {T}, sacrifice source (Lotus
// Blossom's "Add X mana of any one color", X its petal counters) is no unit
// of that census, since the planner cannot price an evaluated-amount source
// whose cost sacrifices it (availForMode's dynSac gate; planCastDynSacSkip).
// A cast only that source and a granted ability (Gift of Paradise's two mana)
// pay is therefore reached by lowering, not offered on gorge's autopay priority
// list, though the census counted the Blossom and called it planned (tail-74
// pair 6, decision 502). The census's other-sacrifice units are an upper bound
// (addOtherSacCensusAvail): a cast only they pay needs the dry plan, which
// names a distinct victim per step (two altars and one creature beside a
// granted source that funds the cast are no planned payment, CR 601.2g-h).
func (s *State) affordsUngrantedCast(k *prep, p uint8, o Obj, cost *kop.ManaCost, d payDesc) bool {
	av, ms := s.ungrCastCensus(k, p)
	if !s.affordsUngrantedAv(k, p, cost, NoSlot, d, &av, &ms) {
		return false
	}
	if k.gen == nil || !k.gen.uses.has(useSacOtherPlan) {
		return true
	}
	av, ms = s.ungrCastSources(k, p)
	return s.affordsUngrantedAv(k, p, cost, NoSlot, d, &av, &ms) || s.otherSacCastPlans(k, p, o, cost)
}

// ungrCastCensus is the planner census affordsUngrantedCast prices over:
// ungrCastSources and the planned other-sacrifice sources' units.
func (s *State) ungrCastCensus(k *prep, p uint8) (avail, modalSet) {
	av, ms := s.ungrCastSources(k, p)
	s.addOtherSacCensusAvail(k, p, &av) // a cast's plan prices the other-sacrifice source (mana_othersac.go)
	return av, ms
}

// ungrCastSources is the pool and the printed mana sources a cast's planner
// census holds, a dynamic sacrifice source and an all-at-once source under a
// reflect carrier (eachPickSources: gorge's census defers it) excepted.
func (s *State) ungrCastSources(k *prep, p uint8) (avail, modalSet) {
	skip := s.eachPickSources(k, p)
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		if pc := s.manaPCGen(k, sl); pc.manaDyn && pc.manaSac {
			skip = skip.with(sl)
		}
	}
	var av avail
	var ms modalSet
	s.ungrAvail(k, p, NoSlot, &av, &ms, skip)
	return av, ms
}
