package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// powerUpManaObj returns a's mana cost after CR 702.193b: if the permanent
// entered its current zone this turn, its own mana cost reduces the ability
// cost. The caller guards on anyPowerUp and CostPowerUp; this body is cold.
//
//go:noinline
func (s *State) powerUpManaObj(k *prep, o Obj, a *kop.Ability) kop.ManaCost {
	return s.powerUpReduced(k, o, a.Cost.Mana)
}

// powerUpReduced applies CR 702.193b to c: on the turn the permanent entered
// its current zone, its own mana cost reduces the ability cost. gorge applies
// this before the cost statics (legal_walk_battlefield.go powerUpReduceCost,
// then offerCastable), so abReduced composes on the result.
//
//go:noinline
func (s *State) powerUpReduced(k *prep, o Obj, c kop.ManaCost) kop.ManaCost {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || !s.g.zoneTurn.has(o) {
		return c
	}
	// the mana cost is the characteristic now in force: a copy's is the copied card's (CR 707.2)
	return powerUpReduce(c, k.t.Cards[s.cardAt(ob.slot)].Cost)
}

// powerUpReduce is gorge's powerUpReduceCost: generic reduces generic
// (clamped); each coloured or colourless symbol reduces its matching symbol
// first, and any excess of that symbol reduces generic (CR 118.7). Hybrid and
// {X} symbols in either cost are untouched.
func powerUpReduce(c, red kop.ManaCost) kop.ManaCost {
	if red.Generic > 0 {
		n := int(red.Generic)
		if n > int(c.Generic) {
			n = int(c.Generic)
		}
		c.Generic -= uint8(n)
	}
	for i := 0; i < kop.NumManaKinds; i++ {
		n := int(red.Pips[i])
		if n == 0 {
			continue
		}
		matched := n
		if matched > int(c.Pips[i]) {
			matched = int(c.Pips[i])
		}
		c.Pips[i] -= uint8(matched)
		left := n - matched
		if left > int(c.Generic) {
			left = int(c.Generic)
		}
		c.Generic -= uint8(left)
	}
	return c
}

// powerUpWitnessAffords reports whether av pays gorge's autopay planner
// witness for Power-up ability a of slot sl (powerUpWitness). The caller
// guards on anyPowerUp and CostPowerUp and on the pool not paying; this body
// is cold.
//
//go:noinline
func (s *State) powerUpWitnessAffords(k *prep, sl Slot, a *kop.Ability, ctrl uint8, av *avail) bool {
	c := s.powerUpWitness(k, a, sl, ctrl)
	return s.abilityAffords(k, sl, a, &c, av)
}

// powerUpWitness is the cost gorge's autopay planner prices for Power-up
// ability a of slot sl: the printed cost with the ability's own reduction
// and the cost statics composed (planAbilityPaymentLifePips:
// ownReduceCostOffer, then offerCostFor; CR 601.2f), but never the CR
// 702.193b entry-turn reduction. Hulk, Gamma Goliath's {3} lowers it.
//
//go:noinline
func (s *State) powerUpWitness(k *prep, a *kop.Ability, sl Slot, ctrl uint8) kop.ManaCost {
	m := a.Cost.Mana
	if k.gen != nil && k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) && sl != NoSlot {
		m = s.ownReducedBF(k, a, sl, ctrl)
	}
	if k.gen != nil && k.gen.anyAbReduce {
		m = s.abReduced(k, a, sl, ctrl, m)
	}
	return m
}

// powerUpPlanRejected reports whether gorge's payment planner witness for a
// Power-up ability is rejected at payment. planAbilityPayment prices the
// PRINTED cost (it never calls powerUpReduceCost) while the activation
// charges the reduced one, so paymentPlanCheck sees cost_changed and drops
// the witness; the manual mana window then pays source by source. When the
// reduction is empty the printed cost already is the charge, the witness
// stands, and the planner's order is the one to follow.
//
//go:noinline
func (s *State) powerUpPlanRejected(k *prep, a *kop.Ability) bool {
	return s.powerUpManaObj(k, s.bld().e.obj, a) != a.Cost.Mana
}

// powerUpArmsNone reports whether gorge arms no autopay witness for the
// Power-up activation being built: armAbilityAutoPay reads the charge before
// the pip asks (castPaymentMana) and arms only a cost with generic or coloured
// mana (abilityCostNeedsMana). A charge the CR 702.193b reduction and the cost
// statics left hybrid-only (Abomination's {5}{R/G}{R/G} less its own {3}{R/G}
// and Hulk, Gamma Goliath's {3}) pays through the manual window, source by
// source in battlefield order. The caller guards on anyPowerUp and
// CostPowerUp; this body is cold.
//
//go:noinline
func (s *State) powerUpArmsNone(k *prep) bool {
	s.bld().bfl |= bflCostComp
	c, _ := s.buildCostPre(k, false, true)
	s.bld().bfl &^= bflCostComp
	return c.Generic == 0 && c.Pips == [kop.NumManaKinds]uint8{}
}

// PowerUpState reports a permanent's once bit and entry-turn bit. It is a
// cold query for the oracle's tally; it has no mutable state.
func PowerUpState(s *State, o Obj) (used, entered bool, zone kop.Zone) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return false, false, ob.zone
	}
	if k := s.k(); k != nil {
		cd := &k.t.Cards[ob.card]
		for i := range cd.Abilities {
			a := &cd.Abilities[i]
			if a.Kind == kop.AbActivated && a.Cost.Flags&kop.CostPowerUp != 0 {
				return s.onceUsed(k, ob.slot, a), s.g.zoneTurn.has(o), ob.zone
			}
		}
	}
	return s.perms[ob.slot].flags&pfOnceGame != 0, s.g.zoneTurn.has(o), ob.zone
}

// PowerUpCost reports o's power-up ability index and its CR 702.193b cost
// (the reduced cost on the entry turn, the printed cost otherwise). It is a
// cold query for the oracle's occurrence tally; it has no mutable state.
//
//go:noinline
func PowerUpCost(s *State, o Obj) (int, kop.ManaCost, bool) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return 0, kop.ManaCost{}, false
	}
	k := s.k()
	if k == nil {
		return 0, kop.ManaCost{}, false
	}
	cd := &k.t.Cards[ob.card]
	for i := range cd.Abilities {
		if cd.Abilities[i].Kind == kop.AbActivated && cd.Abilities[i].Cost.Flags&kop.CostPowerUp != 0 {
			return i, s.powerUpManaObj(k, o, &cd.Abilities[i]), true
		}
	}
	return 0, kop.ManaCost{}, false
}
