package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// affordsUngrantedCast is affordsUngrantedFor for the spell cast gorge's
// planner would have to price: a dynamic {T}, sacrifice source (Lotus
// Blossom's "Add X mana of any one color", X its petal counters) is no unit
// of that census, since the planner cannot price an evaluated-amount source
// whose cost sacrifices it (availForMode's dynSac gate; planCastDynSacSkip).
// A cast only that source and a granted ability (Gift of Paradise's two mana)
// pay is therefore reached by lowering, not offered on gorge's autopay priority
// list, though the census counted the Blossom and called it planned (tail-74
// pair 6, decision 502).
func (s *State) affordsUngrantedCast(k *prep, p uint8, cost *kop.ManaCost, d payDesc) bool {
	av, ms := s.ungrCastCensus(k, p)
	return s.affordsUngrantedAv(k, p, cost, NoSlot, d, &av, &ms)
}

// ungrCastCensus is the planner census affordsUngrantedCast prices over: the
// pool and the printed mana sources, a dynamic sacrifice source and an all-at-once source
// under a reflect carrier (eachPickSources: gorge's census defers it) excepted.
func (s *State) ungrCastCensus(k *prep, p uint8) (avail, modalSet) {
	skip := s.eachPickSources(k, p).or(s.dynFiltIntrinsic(k, p)) // (a dynamic paid filter's intrinsic unit goes with its census gap: mana_dynfiltintrinsic.go)
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
