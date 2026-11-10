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
	if s.rbAny(p & 1) {
		s.restrAvail(k, p, c, s.restrOriginOfObj(o), &av)
	}
	cost := s.altAddReduced(k, s.buildCost(k), s.altAddManaPart(k, c))
	return av.total == 0 || !av.affords(&cost, pipMaskOf(&cost), 0)
}
