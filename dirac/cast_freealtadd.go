package dirac

// freeAltAddLowers: a free may-play cast (Maralen, Fae Ascendant's permission)
// of card c, whose either-or additional cost still has to be paid (CR 118.9d),
// is one gorge's witness lowers: no part without mana can be paid, so gorge
// prices the cast at its {0} plus the part that has mana (rules
// potentialAltAddPlan), and the floating pool does not already pay that (a
// pool that does offers the cast with no tap). lowerWitness's
// witnessAtCheapest then prices the same total (CR 601.2b, 601.2f).
func (s *State) freeAltAddLowers(k *prep, p uint8, c CardID, o Obj) bool {
	if !k.t.Cards[c].HasAltAdd || s.altAddFreePart(k, p, c, o) {
		return false
	}
	var av avail
	s.availPool(p, &av)
	if s.rbAny(SeatIx(p)) {
		s.restrAvail(k, p, c, s.restrOriginOfObj(o), &av)
	}
	cost := s.altAddReduced(k, s.buildCost(k), s.altAddManaPart(k, c))
	return av.total == 0 || !av.affords(&cost, pipMaskOf(&cost), 0)
}

// altFreePartUnpooled: a free hand cast of card c, which gorge offers as an
// alternative cost (Omniscience's permission), still owes an either-or additional cost (CR 118.9d), no
// part without mana can be paid, and the floating pool does not pay the part
// with mana. gorge's surfaces offer such a cast only when the pool pays it:
// the potential planner leaves an alternative-cost play beside the printed
// cast unpriced, so no witness lowers it (CR 601.2b, 601.2f).
func (s *State) altFreePartUnpooled(k *prep, p uint8, c CardID, o Obj) bool {
	if !k.t.Cards[c].HasAltAdd || s.altAddFreePart(k, p, c, o) {
		return false
	}
	var av avail
	s.availPool(p, &av)
	if s.rbAny(SeatIx(p)) {
		s.restrAvail(k, p, c, s.restrOriginOfObj(o), &av)
	}
	cost := s.adjustCostGen(k, p, c, o, s.altAddManaPart(k, c))
	return av.total == 0 || !av.affords(&cost, pipMaskOf(&cost), 0)
}
