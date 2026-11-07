package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// abReduced returns activated ability a's mana cost c after the
// StatReduceAbility statics of its battlefield source sl
// (ability): every static of the activator ctrl whose condition holds, whose
// filter matches the source and, for B bit 0 only for an Exhaust ability and
// for B bit 2 only for a Power-up ability, subtracts its amount from the
// generic part, floored at zero after each
// (CR 601.2f, 602.2b; gorge costModifiersCompose after the ability's own
// reduction, pay.ApplyTo), in battlefield entry order (eachStaticTS). The offer, the planner witness and the charge all
// price this cost. The callers guard on anyAbReduce; this body is cold.
//
//go:noinline
func (s *State) abReduced(k *prep, a *kop.Ability, sl Slot, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	if DebugNoAbReduce || a.Kind != kop.AbActivated && a.Kind != kop.AbMana || sl == NoSlot {
		return c
	}
	if a.ActZone != kop.ZoneNone && a.ActZone != kop.ZoneBattlefield {
		return c // AffectedZone$ Battlefield: a hand or graveyard ability is not reduced
	}
	g := int(c.Generic)
	s.eachStaticTS(k, sclsReduce, func(st statSrc) bool {
		sa := &st.a.Static
		if st.a.Kind != kop.AbStatic || sa.Kind != kop.StatReduceAbility || st.ctrl != ctrl {
			return true
		}
		if sa.B&kop.ReduceAbUnlock != 0 {
			return true // an unlock-cost reduction never lowers an ability
		}
		if sa.B&1 != 0 && !a.Exhaust {
			return true
		}
		if sa.B&kop.ReduceAbPowerUp != 0 && a.Cost.Flags&kop.CostPowerUp == 0 {
			return true // a Power-up reduction never lowers another ability
		}
		if !s.statCond(k, st) || !s.matchPermGen(k, sa.Filter, sl, st.ctrl, st.sl, layerFull) {
			return true
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
	c.Generic = uint8(g)
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
	for pp := uint8(0); pp < 2; pp++ {
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.g.emb[pp][i]-1]
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
// permanent. AbReducedPriced counts the pricings a reduction changed.
//
//go:noinline
func (s *State) abReducedCharge(k *prep, a *kop.Ability, o Obj, ctrl uint8, c kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return c
	}
	r := s.abReduced(k, a, ob.slot, ctrl, c)
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
