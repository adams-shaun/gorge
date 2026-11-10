package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The cost objects of an activation are disjoint from the mana that pays
// it (sacmana). gorge prices an activated ability's mana with the
// candidates of its sacrifice and tap-another parts held back
// (engine/rules/potential_plan.go planComposedCost): a Treasure sacrificed
// for {1} is gone, so it is not also the token Fountainport's cost
// sacrifices (planLeavesCostPayable's "gone"), and a creature tapped for
// mana is not also the untapped creature a tapXType cost taps
// (planEachReservation's tap reservation, PlanPaymentCostWithout's
// "tapped"). A permanent tapped for its own (non-sacrificing) mana can
// still be sacrificed (CR 602.2b; Makeshift Munitions' artifact land).
//
// scriptGapAvail adds sl's contribution to the gaps walk (availScript), the
// units a scripted prefix search would tap it for. costPartDisjoint reads it
// to withhold a tapX part's candidates from that mana: a permanent tapped for
// its mana is not also the untapped creature the part taps
// (CR 601.2g-h; gorge planEachReservation's tap reservation).
func (s *State) scriptGapAvail(k *prep, p uint8, sl Slot, a *avail) {
	c := s.cardAt(sl)
	if s.scriptGaps(k, p).has(sl) {
		out := s.eachColorOut(k, p, sl)
		for col := 0; col < 5; col++ {
			if out&(1<<col) != 0 {
				a.addUnits(1<<col, 1)
			}
		}
	}
	if s.scriptRestr(k, p).has(sl) {
		if k.gen.has(c, gfDeferred) {
			a.addUnits(1<<kop.R, 1)
		} else {
			ab := &k.t.Cards[c].Abilities[k.gen.restr(c)]
			a.addUnits(1<<bits.TrailingZeros8(uint8(ab.ManaColors)), max(ab.ManaN, 1))
		}
	}
	if s.scriptTapsDefer(k, p).has(sl) {
		pc := s.manaPCGen(k, sl)
		a.addUnits(s.srcMaskAt(pc, sl), srcYield(pc))
	}
	if k.ctrMana && s.scriptCtr(k, p).has(sl) {
		pc := &k.cards[c]
		if k.t.Cards[c].Abilities[pc.manaAb].ManaEach {
			for col := 0; col < kop.NumManaKinds; col++ {
				if uint8(pc.manaCol)&(1<<col) != 0 {
					a.addUnits(1<<col, pc.manaN)
				}
			}
		} else {
			a.addUnits(uint8(pc.manaCol), pc.manaN)
		}
	}
	if s.scriptTapXModelled(k, p).has(sl) {
		pc := &k.cards[c]
		a.addUnits(uint8(pc.manaCol), pc.manaN)
	}
}

// costManaDisjoint reports whether some N candidates of a's Sac or TapX
// part leave av (the mana walk's availability, the source's own {T}
// already taken out) able to pay a's mana. A candidate whose use takes
// mana away (a sacrificed Treasure, a tapped mana creature) is reserved
// only when the free candidates do not cover N; each reservation of the
// remaining count is tried, up to gorge's planReservationLimit.
func (s *State) costManaDisjoint(k *prep, sl Slot, a *kop.Ability, av *avail, pm uint8) bool {
	return s.costPartDisjoint(k, s.perms[sl].ctrl, sl, &a.Cost, &a.Cost.Mana, av, pm)
}

// costPartDisjoint is costManaDisjoint for p paying c's Sac or TapX part beside
// mana, from source sl (NoSlot: a spell).
func (s *State) costPartDisjoint(k *prep, p uint8, sl Slot, c *kop.Cost, mana *kop.ManaCost, av *avail, pm uint8) bool {
	return s.costPartDisjointD(k, p, sl, c, mana, av, pm, nil)
}

// costPartDisjointD is costPartDisjoint with rd naming the payment whose
// restricted sources av holds (restrSrcAvail's rkExt units, cost_fixedtaps.go):
// such a source is a tap candidate whose use takes its restricted mana away,
// never a free one (CR 601.2g-h; gorge planReserving).
func (s *State) costPartDisjointD(k *prep, p uint8, sl Slot, c *kop.Cost, mana *kop.ManaCost, av *avail, pm uint8, rd *payDesc) bool {
	f := c.Flags
	tapX := f&kop.CostTapX != 0
	selfTap := f&kop.CostTap != 0
	var cons [MaxPerm]Slot
	nc, free := 0, 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		x := m.first()
		if tapX && (s.perms[x].flags&pfTapped != 0 || selfTap && x == sl) {
			continue
		}
		if !s.matchPerm(k, c.Filter, x, p, sl) {
			continue
		}
		if x == sl && selfTap {
			free++ // its {T} already took its mana out of av
			continue
		}
		pc := s.manaPCAt(k, x)
		sacOnly := s.canSacOnlyNow(k, x, pc)
		if !s.canTapForManaPC(k, x, pc) && !sacOnly || !tapX && !(pc.manaSac || sacOnly) {
			if tapX && rd != nil && nc < len(cons) {
				if _, ok := s.restrTapUnit(k, p, x, rd); ok {
					cons[nc] = x
					nc++
					continue
				}
			}
			if tapX && k.gen != nil && s.g.availX&availGaps != 0 {
				// A scripted gap source's mana is in the gaps walk without a
				// plain tap profile (Bloom Tender's each-colour ability): the
				// unit it adds is the same permanent the tapX part taps
				// (CR 601.2g-h, gorge planEachReservation), so it is not a
				// free candidate.
				var gap avail
				s.scriptGapAvail(k, p, x, &gap)
				if gap.total > 0 {
					if nc == len(cons) {
						free++ // beyond the bound: counted free (never reached in the corpus pools)
						continue
					}
					cons[nc] = x
					nc++
					continue
				}
			}
			free++
			continue
		}
		if nc == len(cons) {
			free++ // beyond the bound: counted free (never reached in the corpus pools)
			continue
		}
		cons[nc] = x
		nc++
	}
	need := int(c.N) - free
	if need <= 0 {
		return true
	}
	if need > nc {
		return false
	}
	// each need-subset of cons, in slot order (lexicographic combinations)
	var idx [MaxPerm]uint8
	for i := 0; i < need; i++ {
		idx[i] = uint8(i)
	}
	for tries := 0; tries < planReservationLimit; tries++ {
		r := *av
		for i := 0; i < need; i++ {
			x := cons[idx[i]]
			pc := s.manaPCAt(k, x)
			if kind := s.takeRestrUnit(k, p, x, rd, &r); kind == rkWiden || kind == rkExt && !s.canTapForManaPC(k, x, pc) {
				continue // its census unit is not in r (widened) or it has none
			}
			s.removeSrc(k, &r, x, pc)
			if tapX && k.gen != nil && s.g.availX&availGaps != 0 {
				var gap avail
				s.scriptGapAvail(k, p, x, &gap)
				for j := 0; j < gap.n; j++ {
					r.removeWide(gap.mask[j], gap.cnt[j])
				}
			}
		}
		if r.affords(mana, pm, 0) {
			return true
		}
		i := need - 1
		for i >= 0 && int(idx[i]) == nc-need+i {
			i--
		}
		if i < 0 {
			return false
		}
		idx[i]++
		for j := i + 1; j < need; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
	return false
}

// planReservationLimit is gorge's bound on the candidate assignments
// planComposedCost tries (potential_plan.go).
const planReservationLimit = 64

// manaPCAt is the mana facts of the permanent at sl (manaPCGen on a
// generated table).
func (s *State) manaPCAt(k *prep, sl Slot) *pcard {
	if k.gen != nil {
		return s.manaPCGen(k, sl)
	}
	return &k.cards[s.cardAt(sl)]
}

// sacShort reverses a cast or activation whose sacrifice part has fewer
// candidates left than it needs once the CR 601.2g mana is paid (Deadly
// Dispute whose only artifact was the Treasure sacrificed for its {B}):
// gorge sacAsk's "sacrifice cost no longer payable" abortCast (CR 733.1).
// The mana stays in the pool, as gorge leaves it.
func (s *State) sacShort(k *prep, d *Decision) {
	d.N = 0
	s.reverseCast(k)
}

// tapForSacKept is tapFor for an activation whose cost sacrifices
// permanents (CR 601.2g before 601.2h): gorge's witness for it
// (potential_plan.go planComposedCost) is the plain plan when that leaves
// N sacrifice candidates it did not consume (planLeavesCostPayable's
// "gone": a Treasure sacrificed for mana), else the plan with the first
// candidate assignment, in battlefield order, whose tap-and-sacrifice
// members are withheld from the mana (planEachReservation, up to
// planReservationLimit tries). src is the ability's source slot.
func (s *State) tapForSacKept(k *prep, p uint8, cost *kop.ManaCost, x int, a *kop.Ability, src Slot) {
	var pk planner
	pk.dry = true
	s.tapForPK(k, p, cost, x, NoObj, false, false, &pk)
	if !pk.found {
		s.tapFor(k, p, cost, x, NoObj) // nothing owed (or no plan: the offer's own panic)
		return
	}
	// the sacrifice candidates in battlefield (entry) order
	var cands [MaxPerm]Slot
	nc := 0
	for m := s.ctrlM[p]; m.any(); m = m.rest() {
		x := m.first()
		if a.Cost.Flags&kop.CostTap != 0 && x == src || !s.matchPerm(k, a.Cost.Filter, x, p, src) {
			continue
		}
		cands[nc] = x
		nc++
	}
	for i := 1; i < nc; i++ {
		for j := i; j > 0 && s.perms[cands[j]].ts < s.perms[cands[j-1]].ts; j-- {
			cands[j], cands[j-1] = cands[j-1], cands[j]
		}
	}
	n := int(a.Cost.N)
	if s.sacLeft(&pk, &cands, nc) >= n || n > nc || n > len(cands) {
		s.tapFor(k, p, cost, x, NoObj)
		return
	}
	var idx [MaxPerm]uint8
	for i := 0; i < n; i++ {
		idx[i] = uint8(i)
	}
	for tries := 0; tries < planReservationLimit; tries++ {
		var r planner
		r.dry = true
		for i := 0; i < n; i++ {
			r.kept.set(cands[idx[i]])
		}
		kept := r.kept
		s.tapForPK(k, p, cost, x, NoObj, false, false, &r)
		if r.found && s.sacLeft(&r, &cands, nc) >= n {
			var w planner
			w.kept = kept
			s.tapForPK(k, p, cost, x, NoObj, false, false, &w)
			return
		}
		i := n - 1
		for i >= 0 && int(idx[i]) == nc-n+i {
			i--
		}
		if i < 0 {
			break
		}
		idx[i]++
		for j := i + 1; j < n; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
	s.tapFor(k, p, cost, x, NoObj)
}

// sacLeft counts the candidates pk's selected plan does not sacrifice for
// mana.
func (s *State) sacLeft(pk *planner, cands *[MaxPerm]Slot, nc int) int {
	left := nc
	for i := 0; i < pk.bestN; i++ {
		u := pk.bestSeq[i]
		if !pk.types[pk.bestTy[i]].sac {
			continue
		}
		for j := 0; j < nc; j++ {
			if cands[j] == pk.units[u] {
				left--
				break
			}
		}
	}
	return left
}
