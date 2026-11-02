package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// powerUpManaObj returns a's mana cost after CR 702.193b: if the permanent
// entered its current zone this turn, its own mana cost reduces the ability
// cost. The caller guards on anyPowerUp and CostPowerUp; this body is cold.
//
//go:noinline
func (s *State) powerUpManaObj(k *prep, o Obj, a *kop.Ability) kop.ManaCost {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || !s.g.zoneTurn.has(o) {
		return a.Cost.Mana
	}
	return powerUpReduce(a.Cost.Mana, k.t.Cards[ob.card].Cost)
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

// PowerUpState reports a permanent's once bit and entry-turn bit. It is a
// cold query for the oracle's tally; it has no mutable state.
func PowerUpState(s *State, o Obj) (used, entered bool, zone kop.Zone) {
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield {
		return false, false, ob.zone
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
