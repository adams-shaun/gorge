package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// abReduced returns activated ability a's mana cost c after the
// StatReduceAbility statics of its battlefield source sl
// (ability): every static of the activator ctrl whose condition holds, whose
// filter matches the source and, for B bit 0 only for an Exhaust ability,
// for B bit 2 only for a Power-up ability and for B bit 3 only for an Equip
// ability, subtracts its amount from the generic part, floored at zero after
// each, the excess past it paying into {X} (CR 601.2f, 602.2b; gorge costModifiersCompose after the ability's own
// reduction, pay.ApplyTo), in battlefield entry order (eachStaticTS). The
// offer, the planner witness and the view price this cost; a target-gated
// equip reduction (kop.StTargetsFilter2: Cloud, Planet's Champion) is priced
// at the ability's cheapest legal target, the target menu prices each
// candidate (abilityAffordableTargets) and the charge the chosen one
// (abReducedCharge). The callers guard on anyAbReduce; this body is cold.
//
//go:noinline
func (s *State) abReduced(k *prep, a *kop.Ability, sl Slot, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	if DebugNoAbReduce || a.Kind != kop.AbActivated && a.Kind != kop.AbMana || sl == NoSlot {
		return c
	}
	if a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield {
		return c // AffectedZone$ Battlefield: a hand or graveyard ability is not reduced
	}
	if k.gen == nil || !k.gen.anyAbReduce || a.NTargets != 1 || !s.abilityHasTargetReduction(k, a, sl, ctrl) {
		return s.abReducedFor(k, a, sl, ctrl, c, nil, 0)
	}
	// the offer's best case: the cheapest price over the legal targets of the
	// ability's one target slot (two reducers gated on different targets
	// never combine on one equip)
	var buf [MaxOptions]uint8
	pr := s.g.probe
	if pr.kind == 0 {
		s.g.probe = probe{kind: probeAbility, src: sl, ctrl: ctrl}
	}
	n := s.targetCands(k, &a.Targets[0], ctrl, sl, s.perms[sl].obj, &buf)
	s.g.probe = pr
	best := s.abReducedFor(k, a, sl, ctrl, c, nil, 0)
	var one [kop.MaxTargets]uint8
	for i := 0; i < n; i++ {
		one[0] = buf[i]
		if r := s.abReducedFor(k, a, sl, ctrl, c, &one, 1); r.Generic < best.Generic {
			best = r
		}
	}
	return best
}

// abReduceStaticApplies is the per-static gate of a StatReduceAbility static
// st on ability a of the battlefield source sl activated by ctrl: the
// activator, the unlock/Exhaust/Power-up/Equip class, the condition and the
// ValidCard$ filter over the source. A target gate (StTargetsFilter2) is the
// caller's.
func (s *State) abReduceStaticApplies(k *prep, st statSrc, a *kop.Ability, sl Slot, ctrl uint8) bool {
	sa := &st.a.Static
	if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatReduceAbility || st.ctrl != ctrl {
		return false
	}
	if sa.B&kop.ReduceAbUnlock != 0 {
		return false // an unlock-cost reduction never lowers an ability
	}
	if sa.B&1 != 0 && !a.Exhaust { // seat-ok: an ability flag bit
		return false
	}
	if sa.B&kop.ReduceAbPowerUp != 0 && a.Cost.Flags&kop.CostPowerUp == 0 {
		return false // a Power-up reduction never lowers another ability
	}
	if sa.B&kop.ReduceAbEquip != 0 && a.AFlags&kop.AfEquip == 0 {
		return false // an Equip reduction never lowers another ability
	}
	return s.statCond(k, st) && s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull)
}

// abilityHasTargetReduction: some StatReduceAbility static gated on the
// ability's target (StTargetsFilter2) applies to a's source sl and activator
// ctrl, so its price depends on the target (the offer probe and the target
// menu's prune read it).
func (s *State) abilityHasTargetReduction(k *prep, a *kop.Ability, sl Slot, ctrl uint8) bool {
	found := false
	s.eachStaticTS(k, sclsReduce, func(st statSrc) bool {
		if st.a.Static.StFlags&kop.StTargetsFilter2 != 0 && s.abReduceStaticApplies(k, st, a, sl, ctrl) {
			found = true
			return false
		}
		return true
	})
	return found
}

// abReducedFor is abReduced's composition with the targets tgts[0..nt): a
// target-gated static lowers the cost only when one of them matches its
// Filter2 read with the static's source as Self (nt == 0: none applies).
func (s *State) abReducedFor(k *prep, a *kop.Ability, sl Slot, ctrl uint8, c kop.ManaCost, tgts *[kop.MaxTargets]uint8, nt int) kop.ManaCost {
	g, xr := int(c.Generic), int(c.XRed)
	s.eachStaticTS(k, sclsReduce, func(st statSrc) bool {
		sa := &st.a.Static
		if !s.abReduceStaticApplies(k, st, a, sl, ctrl) {
			return true
		}
		if sa.StFlags&kop.StTargetsFilter2 != 0 {
			hit := false
			for i := 0; i < nt && !hit; i++ {
				hit = s.costTargetMatchSrc(k, sa.Filter2, ctrl, tgts[i], st.sl)
			}
			if !hit {
				return true
			}
		}
		n := int(sa.A)
		if sa.StFlags&kop.StDynamic != 0 {
			n = s.evalV(k, sa.VA, s.ctxStatic(st))
		}
		if n <= 0 {
			return true
		}
		floor := int(sa.B>>kop.ReduceAbMinShift) & 15
		if floor == 0 || DebugNoMinManaFloor {
			if g < n && c.X != 0 {
				xr = min(xr+n-g, 255) // CR 601.2f: the excess pays into {X}
			}
			g = max(g-n, 0)
			return true
		}
		// MinMana$ caps this reduction: the cost never drops below
		// min(floor, its mana value before it) (gorge pay.ApplyTo).
		c.Generic = uint8(g)
		before := c.MV()
		g = max(g-n, 0)
		c.Generic = uint8(g)
		if fl := min(floor, before); c.MV() < fl {
			g += fl - c.MV()
			AbFloorRaised.Add(1)
		}
		return true
	})
	c.Generic, c.XRed = uint8(g), uint8(xr)
	return c
}

// eachStaticTS is eachStaticC in gorge's composition order: the permanents'
// statics by perm.ts ascending (the order of the battlefield zone list that
// gorge's scanActiveStatics walks; a slot reused after a departure breaks
// slot order), then the emblems'. A floored reduction (MinMana$) does not
// commute with the others, so abReduced needs this order. Cold: it selects
// the next-oldest slot each round (no sort, no allocation).
func (s *State) eachStaticTS(k *prep, cls uint16, f func(st statSrc) bool) {
	var lastTs uint16
	lastSl := NoSlot
	for {
		best := NoSlot
		for m := s.statM; m.any(); m = m.rest() {
			sl := m.first()
			if k.gen.sCls[s.cardAt(sl)]&cls == 0 {
				continue
			}
			ts := s.perms[sl].ts
			if lastSl != NoSlot && (ts < lastTs || ts == lastTs && sl <= lastSl) {
				continue
			}
			if best == NoSlot || ts < s.perms[best].ts || ts == s.perms[best].ts && sl < best {
				best = sl
			}
		}
		if best == NoSlot {
			break
		}
		lastTs, lastSl = s.perms[best].ts, best
		if s.genLost(k, best) {
			continue
		}
		p := &s.perms[best]
		cd := &k.t.Cards[s.cardAt(best)]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbStatic || a.Kind == kop.AbReplacement {
				if !f(statSrc{a: a, sl: best, ctrl: p.ctrl, obj: p.obj}) {
					return
				}
			}
		}
	}
	for pp := uint8(0); pp < s.nSeats(); pp++ {
		if s.out(pp) {
			continue
		}
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.embIdx(pp, i)-1]
			for j := range e.Abilities {
				a := &e.Abilities[j]
				if a.Kind == kop.AbStatic {
					if !f(statSrc{a: a, sl: NoSlot, ctrl: pp, obj: NoObj}) {
						return
					}
				}
			}
		}
	}
}

// abReducedCharge is abReduced for an ability stack entry being priced
// (buildCost0): the source is the entry's object, if it is a battlefield
// permanent, and a target-gated reduction reads the entry's chosen targets
// (CR 601.2f: they are announced at bsTargets, before the charge).
// AbReducedPriced counts the pricings a reduction changed.
//
//go:noinline
func (s *State) abReducedCharge(k *prep, a *kop.Ability, o Obj, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return c
	}
	if DebugNoAbReduce || a.Kind != kop.AbActivated && a.Kind != kop.AbMana || a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield {
		return c
	}
	e := &s.bld().e
	r := s.abReducedFor(k, a, ob.slot, ctrl, c, &e.tgt, int(e.nt))
	if r != c {
		AbReducedPriced.Add(1)
	}
	return r
}

// DebugNoAbReduce drops the reduction (TestReduceabMutation proves the
// pool activates a reduced ability).
var DebugNoAbReduce bool

// AbReducedPriced counts the ability pricings a StatReduceAbility lowered
// (read by TestReduceabLockstep; nothing in the engine reads it).
var AbReducedPriced atomic.Int64

// DebugNoMinManaFloor drops the MinMana$ floor (TestReduceminmanaMutation).
var DebugNoMinManaFloor bool

// AbFloorRaised counts the pricings where a MinMana$ floor raised the
// generic part back up (read by TestReduceminmanaLockstep).
var AbFloorRaised atomic.Int64

// abPlannedCost is the mana cost gorge's ability planner prices
// (planAbilityPayment): the printed cost with the cost statics composed
// (offerCostFor), but never Power-up's entry-turn reduction (CR 702.193b),
// which the planner does not apply. sl is the source's battlefield slot and
// ctrl its activator. The offer gate's printed-cost witness and the
// activation's armed plan both read it, so Hulk, Gamma Goliath's {3} less
// prices the plan as it prices the charge (CR 601.2f, 602.2b).
func (s *State) abPlannedCost(k *prep, a *kop.Ability, sl Slot, ctrl uint8) kop.ManaCost {
	if k.gen == nil || !k.gen.anyAbReduce {
		return a.Cost.Mana
	}
	return s.abReduced(k, a, sl, ctrl, a.Cost.Mana)
}
