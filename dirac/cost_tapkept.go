package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An activation whose cost taps other permanents (Heap Gate: "{1}, {T}, Tap an
// untapped Gate") is paid by a witness gorge arms as the activation begins, before
// the tap election, from the census with every candidate still untapped
// (armAbilityAutoPay, planReserving). When that plan leaves N candidates it did not
// tap it stands, and the election that follows may pick any of them: the plan runs
// unchanged unless the elected permanents include one of its sources, and only then
// does gorge plan again without the committed permanents (rearmAutoPay). Otherwise
// the first assignment of N candidates, in battlefield order, held back from the mana
// is the witness (planEachReservation, up to planReservationLimit tries).

// tapXPickedSlot reports whether the build's tap election holds sl.
func (s *State) tapXPickedSlot(sl Slot) bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] == pkTapX && s.objs[s.bld().picks[i]].zone == kop.ZoneBattlefield && s.objs[s.bld().picks[i]].slot == sl {
			return true
		}
	}
	return false
}

// planUses reports whether pk's selected plan taps sl.
func (pk *planner) planUses(sl Slot) bool {
	for i := 0; i < pk.bestN; i++ {
		if pk.units[pk.bestSeq[i]] == sl {
			return true
		}
	}
	return false
}

// planOverlapsPicks reports whether pk's plan taps a permanent the build's tap
// election holds.
func (s *State) planOverlapsPicks(pk *planner) bool {
	for i := 0; i < pk.bestN; i++ {
		if s.tapXPickedSlot(pk.units[pk.bestSeq[i]]) {
			return true
		}
	}
	return false
}

// tapForTapKept is tapFor for an activation whose cost taps other permanents. The
// elected permanents are tapped on entry (holdCostTaps, saved): the witness is
// planned with them released, then paid unless it taps one of them.
func (s *State) tapForTapKept(k *prep, p uint8, cost *kop.ManaCost, x int, a *kop.Ability, src Slot, saved *[len(build{}.picks)]uint8) {
	s.releaseCostTaps(k, saved)
	var pk planner
	pk.dry = true
	s.tapForPK(k, p, cost, x, NoObj, false, false, &pk)
	if !pk.found {
		s.retakeCostTaps(saved)
		s.tapFor(k, p, cost, x, NoObj) // nothing owed (or no plan: the offer's own panic)
		return
	}
	skip := NoSlot
	if a.Cost.Flags&kop.CostTap != 0 {
		skip = src
	}
	cands, nc := s.tapCandsByEntry(k, p, a.Cost.Filter, src, skip)
	n := int(a.Cost.N)
	if pk.unplannedCount(&cands, nc) >= n || n > nc {
		s.payArmed(k, p, cost, x, &pk, saved, slotMask{})
		return
	}
	if held, r, ok := s.firstHeldPlan(k, p, cost, x, NoObj, false, &cands, nc, n); ok {
		s.payArmed(k, p, cost, x, &r, saved, held)
		return
	}
	s.retakeCostTaps(saved)
	s.tapFor(k, p, cost, x, NoObj)
}

// tapCandsByEntry collects p's untapped permanents matching f (matchPerm with
// src), skip left out, in battlefield (entry) order: the order gorge's
// planEachReservation assigns the tap candidates in.
func (s *State) tapCandsByEntry(k *prep, p uint8, f kop.FilterID, src, skip Slot) (cands [MaxPerm]Slot, nc int) {
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		c := m.first()
		if s.perms[c].flags&pfTapped != 0 || c == skip || !s.matchPerm(k, f, c, p, src) {
			continue
		}
		cands[nc] = c
		nc++
	}
	for i := 1; i < nc; i++ {
		for j := i; j > 0 && s.perms[cands[j]].ts < s.perms[cands[j-1]].ts; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	return cands, nc
}

// unplannedCount is how many of the nc candidates pk's plan does not tap.
func (pk *planner) unplannedCount(cands *[MaxPerm]Slot, nc int) int {
	left := 0
	for i := 0; i < nc; i++ {
		if !pk.planUses(cands[i]) {
			left++
		}
	}
	return left
}

// firstHeldPlan is gorge's planEachReservation: the first assignment of n of the
// nc candidates (lexicographic, up to planReservationLimit) that, held back from
// the mana, leaves a plan for cost. It returns the held candidates and that dry
// plan, the candidates untapped again; ok is false when no assignment has one.
func (s *State) firstHeldPlan(k *prep, p uint8, cost *kop.ManaCost, x int, excl Obj, anyColor bool, cands *[MaxPerm]Slot, nc, n int) (held slotMask, r planner, ok bool) {
	var idx [MaxPerm]uint8
	for i := 0; i < n; i++ {
		idx[i] = uint8(i)
	}
	for tries := 0; tries < planReservationLimit; tries++ {
		held = slotMask{}
		for i := 0; i < n; i++ {
			held.set(cands[idx[i]])
		}
		s.holdSlots(held, true)
		r = planner{dry: true}
		s.tapForPK(k, p, cost, x, excl, false, anyColor, &r)
		s.holdSlots(held, false)
		if r.found {
			return held, r, true
		}
		if !nextCombination(&idx, n, nc) {
			break
		}
	}
	return slotMask{}, planner{}, false
}

// nextCombination advances idx, an n-subset of 0..nc-1 in increasing order, to
// the next one in lexicographic order; it reports false after the last.
func nextCombination(idx *[MaxPerm]uint8, n, nc int) bool {
	i := n - 1
	for i >= 0 && int(idx[i]) == nc-n+i {
		i--
	}
	if i < 0 {
		return false
	}
	idx[i]++
	for j := i + 1; j < n; j++ {
		idx[j] = idx[j-1] + 1
	}
	return true
}

// holdSlots taps (on) or untaps (off) the permanents of m, which are untapped
// outside the call.
func (s *State) holdSlots(m slotMask, on bool) {
	for ; m.any(); m = m.rest() {
		if on {
			s.perms[m.first()].flags |= pfTapped
		} else {
			s.perms[m.first()].flags &^= pfTapped
		}
	}
}

// payArmed pays with the witness armed (planned dry as pk, with held back from the
// mana) when the election leaves it standing, else with the plan made once the
// elected permanents are committed.
func (s *State) payArmed(k *prep, p uint8, cost *kop.ManaCost, x int, pk *planner, saved *[len(build{}.picks)]uint8, held slotMask) {
	if s.planOverlapsPicks(pk) {
		s.retakeCostTaps(saved)
		s.tapFor(k, p, cost, x, NoObj)
		return
	}
	s.holdSlots(held, true)
	s.tapFor(k, p, cost, x, NoObj)
	s.holdSlots(held, false)
	s.retakeCostTaps(saved)
}
