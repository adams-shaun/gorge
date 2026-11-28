package dirac

import "slices"

// The member choice of a planned other-sacrifice payment (gorge
// pay.fundPlanVictims): the search fixes how MANY units of a class pay, never
// which, so a plan whose key-8 choice leaves a step no victim taps another
// unit of the same class before the plan is refused (CR 601.2g-h).

// planSacFundable reports whether every other-sacrifice unit of the plan
// seq/tys (bn units, colours cols) can name a distinct victim, in run order,
// changing the plan's units where another member choice funds it: gorge's
// search ranks only paying sets some member choice funds (pay.planVictimGate ->
// fundPlanVictims), so a fundable set still wins beside an unfundable
// better-ranked one. It runs for the dry probe and the live pass alike: the
// offer stands or falls on it in gorge.
func (s *State) planSacFundable(k *prep, p uint8, pk *planner, seq, tys, cols *[maxUnits]uint8, bn int) bool {
	for range bn + 1 {
		short, res := s.planSacShort(k, p, pk, seq, tys, bn)
		if short < 0 {
			return true
		}
		if !s.planSacSwap(k, p, pk, seq, tys, cols, bn, short, res) {
			break
		}
	}
	return false
}

// planSacShort is the first step of the plan that can name no victim (-1:
// none) and the victim exclusion set at that step.
func (s *State) planSacShort(k *prep, p uint8, pk *planner, seq, tys *[maxUnits]uint8, bn int) (int, slotMask) {
	res := s.planSacReserved(k, pk, seq, bn)
	for i := 0; i < bn; i++ {
		if !pk.types[tys[i]].sacO {
			continue
		}
		sl := pk.units[seq[i]]
		pc := s.manaPCGen(k, sl)
		ab, ok := s.otherSacAb(k, sl, pc)
		if !ok {
			return i, res
		}
		vic, ok := s.otherSacVictim(k, p, sl, ab, res)
		if !ok {
			return i, res
		}
		res = res.with(vic)
	}
	return -1, res
}

// planSacSwap frees one victim for the unfunded step short, whose exclusion
// set is res (gorge pay.swapPlanMember): among the plan's sources the step
// could sacrifice and whose class has an unused unit it could not, the victim
// ranking picks the one to free, and that class's oldest such unit takes its
// step. The steps return to unit order. It reports false when no source can be
// exchanged.
func (s *State) planSacSwap(k *prep, p uint8, pk *planner, seq, tys, cols *[maxUnits]uint8, bn, short int, res slotMask) bool {
	sl := pk.units[seq[short]]
	ab, ok := s.otherSacAb(k, sl, s.manaPCGen(k, sl))
	if !ok {
		return false
	}
	var cs [MaxPerm]Slot
	var cands slotMask
	for _, x := range cs[:s.sacXCands(k, p, sl, ab, &cs)] {
		cands.set(x)
	}
	kept := pk.kept.or(s.buildOtherSacKept(k))
	var spare [maxUnits]uint8
	var freeable [maxUnits]bool
	for i := 0; i < bn; i++ {
		v := pk.units[seq[i]]
		if !cands.has(v) || kept.has(v) {
			continue
		}
		if u, ok := s.planSacSpare(k, pk, seq, bn, seq[i], cands); ok {
			spare[i], freeable[i] = u, true
			res = res.without(v)
		}
	}
	vic, ok := s.otherSacVictim(k, p, sl, ab, res)
	if !ok {
		return false
	}
	for i := 0; i < bn; i++ {
		if !freeable[i] || pk.units[seq[i]] != vic {
			continue
		}
		if tys[i] == pk.utyp[seq[i]] {
			tys[i] = pk.utyp[spare[i]]
		}
		seq[i] = spare[i]
		sortPlanByUnit(seq, tys, cols, bn)
		return true
	}
	return false
}

// planSacSpare is the oldest unit of unit u's class that the plan seq does
// not tap and that is not among cands.
func (s *State) planSacSpare(k *prep, pk *planner, seq *[maxUnits]uint8, bn int, u uint8, cands slotMask) (uint8, bool) {
	granted := s.planUnitsGranted(k, pk)
	var want [planLimitAlts]lsAlt
	n := s.unitAlts(k, pk, int(u), &want, granted)
	best, found := uint8(0), false
	for v := uint8(0); int(v) < pk.nu; v++ {
		if slices.Contains(seq[:bn], v) || cands.has(pk.units[v]) || !pk.sameMemberType(u, v) {
			continue
		}
		var got [planLimitAlts]lsAlt
		if s.unitAlts(k, pk, int(v), &got, granted) != n || got != want {
			continue
		}
		if !found || s.unitRank(pk.units[v]) < s.unitRank(pk.units[best]) {
			best, found = v, true
		}
	}
	return best, found
}

// sameMemberType reports whether units u and v are one pay.Class by their
// types (sameClassType, and the fields the limit search never meets); their
// alternative lists are compared beside it.
func (pk *planner) sameMemberType(u, v uint8) bool {
	if pk.rkind[u] != pk.rkind[v] {
		return false
	}
	a, b := pk.utyp[u], pk.utyp[v]
	if a == b {
		return true
	}
	x, y := &pk.types[a], &pk.types[b]
	return pk.sameClassType(a, b) && !x.inert && !y.inert && x.pair == 0 && y.pair == 0 &&
		x.sacO == y.sacO && x.sacOnly == y.sacOnly && x.pain == y.pain
}

// sortPlanByUnit puts the plan's first bn steps back in unit order, the
// witness order (gorge materialize), each step keeping its type and colour.
func sortPlanByUnit(seq, tys, cols *[maxUnits]uint8, bn int) {
	for i := 1; i < bn; i++ {
		for j := i; j > 0 && seq[j] < seq[j-1]; j-- {
			seq[j], seq[j-1] = seq[j-1], seq[j]
			tys[j], tys[j-1] = tys[j-1], tys[j]
			cols[j], cols[j-1] = cols[j-1], cols[j]
		}
	}
}

// limitPlanFunded is limitPlan with the limited plan's members chosen again:
// gorge funds the victims of whatever plan its search answers, limited or not.
func (s *State) limitPlanFunded(k *prep, p uint8, pk *planner) {
	s.limitPlan(k, pk)
	s.planSacFundable(k, p, pk, &pk.bestSeq, &pk.bestTy, &pk.bestCol, pk.bestN)
}
