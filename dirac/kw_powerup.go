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

// powerUpBoundReaches reports whether gorge's bounded relaxation of the
// census (potential_plan.go: every uncovered source free, over-producing)
// reaches the printed witness cost of Power-up ability a of slot sl: the
// census's sources plus each untapped filter's output NET of its fee: the
// bounded proof (boundedProof) adds a filter as a paid unit beside the land's
// own census unit and charges its fee as generic on top of the cost
// (paymentPlanRelaxProof), over the best subset of filters, so a filter counts
// only where its output exceeds its fee. Short of it gorge skips the prefix
// search (a numeric necessary condition, not a colour proof), so the
// activation is not offered.
//
// Known gap: gorge drops the bound past relaxedPaidLimit paid units and counts
// a repeatable paid filter once per repeat; this counts each filter once and
// never drops the bound, so four or more filters (or a repeatable net-positive
// one) beside a Power-up can still part. Neither is measured.
//
//go:noinline
func (s *State) powerUpBoundReaches(k *prep, sl Slot, a *kop.Ability, ctrl uint8, av *avail) bool {
	n := av.total
	for f := s.scriptFilt(k, ctrl); f.any(); f = f.rest() {
		sl := f.first()
		n += max(s.filtUnits(k, sl)-s.filtFee(k, sl), 0)
	}
	c := s.powerUpWitness(k, a, sl, ctrl)
	return n >= c.MV()
}

// powerUpGrantSearchLists reports whether the relaxed (gaps) walk lists a
// Power-up activation the printed-cost witness refuses, for the grant search
// to decide: gorge's prefix search taps the granted mana abilities and its
// node's own priority walk offers the play once the floating pool pays the
// reduced charge (offerPays), so three lands Wrenn and Realmbreaker's grant
// taps fund Goliath, Mass Manipulator's {4}{G} at {3} though the planner's
// printed price is five (CR 702.193b, 605.1a). grantSearch drops the play
// when no path pays it (script_gen.go grantCap), so listing it is the search's
// question, not an offer.
func powerUpGrantSearchLists(s *State, k *prep) bool {
	return s.g.availX&availGaps != 0 && k.gen != nil && len(k.gen.grantPC) > 0
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
// powerUpAbilityMana pays a charge ability whose planner witness is outdated
// (the case arm powerUpPlanRejected approved). gorge arms the printed-cost
// witness whenever the charge still owes generic or coloured mana, hybrid
// symbols or not (armAbilityAutoPay), and re-arms it for the reduced cost,
// tapping its plan's order. A charge the reductions left hybrid-only arms no
// witness at all (powerUpArmsNone), so the manual window pays source by
// source, first useful one in battlefield order, tapping nothing when the
// pool already pays it. It reports whether the activation continues.
//
//go:noinline
func (s *State) powerUpAbilityMana(k *prep, e *stackEntry, a *kop.Ability, sl Slot, saved *[len(build{}.picks)]uint8, hostSl, srcSl Slot, selfUntapped bool) bool {
	cc := s.buildCostCharge(k)
	w := s.powerUpWitness(k, a, sl, e.ctrl)
	if !s.powerUpArmsNone(k) && s.abilityPlanSettles(k, e, w, w, cc, saved) {
		s.tapFor(k, e.ctrl, &cc, int(e.x), NoObj)
		return true
	}
	if !s.windowTapHeldOK(k, e.ctrl, &cc) {
		// the window ran out of sources: gorge aborts the activation
		// (CR 733.1) and keeps the taps and the floated mana, as for the
		// waterbend fallback above (mana_gate.go)
		s.reverseActivation(k, saved, hostSl, srcSl, selfUntapped)
		return false
	}
	return true
}

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

// powerUpOffer is the charge gorge's priority walk prices Power-up
// activation play at (legal_walk_battlefield.go: ownReduceCostOffer, then
// powerUpReduceCost, then the cost statics; CR 702.193b, 601.2f), which the
// floating pool alone pays to offer it. ok is false for any other play.
func (s *State) powerUpOffer(k *prep, play Action) (kop.ManaCost, bool) {
	if k.gen == nil || !k.gen.anyPowerUp || play.Kind() != AkActivate || isGainedMode(play.Mode()) || play.Mode()&0x80 != 0 {
		return kop.ManaCost{}, false
	}
	o := Obj(play.Source())
	if o == NoObj || int(o) >= len(s.objs) || s.objs[o].zone != kop.ZoneBattlefield || s.objs[o].slot == NoSlot {
		return kop.ManaCost{}, false
	}
	sl := s.objs[o].slot
	abs := k.t.Cards[s.cardAt(sl)].Abilities
	if int(play.Mode()) >= len(abs) || abs[play.Mode()].Cost.Flags&kop.CostPowerUp == 0 {
		return kop.ManaCost{}, false
	}
	a := &abs[play.Mode()]
	ctrl := s.perms[sl].ctrl
	m := a.Cost.Mana
	if k.gen.anyOwnReduce && a.OwnReduce != (kop.ValueRef{}) {
		m = s.ownReducedBF(k, a, sl, ctrl)
	}
	m = s.powerUpReduced(k, o, m)
	if k.gen.anyAbReduce {
		m = s.abReduced(k, a, sl, ctrl, m)
	}
	return m, true
}

// offerPays reports gorge's offered check at a grant-search node for a
// Power-up play whose reduced charge differs from the planner's price
// (scriptSearch.visit: the node's priority options list the play): the
// floating pool pays the reduced charge, as the offer gate's poolPays does
// (legal.go). Gorge's planner prices the printed cost, so the leaf pricing
// alone would miss it (Goliath, Mass Manipulator's {4}{G} at {3} from three
// lands Wrenn and Realmbreaker's grant tapped).
func (g *grantSrch) offerPays(t *State) bool {
	return g.hasOffer && t.floatPaysPlay(g.k, g.p, &g.offer, g.d) && !g.waitsForStack(t)
}

// seenPays is a seen grant-search node's success: gorge's offered check
// comes before its seen check (scriptSearch.visit: offered, then seen, then
// priced), and a node folded equal to a seen one can still differ in what
// its pool spends (a ghost unit the play cannot spend, restrFloatAdmits), so
// the reduced charge is offered again here, then the pool alone at the
// node's price.
func (g *grantSrch) seenPays(t *State, cost *kop.ManaCost, cheap bool) bool {
	if g.offerPays(t) {
		g.cheaper = true
		return true
	}
	if t.floatPaysPlay(g.k, g.p, cost, g.d) && !g.waitsForStack(t) {
		g.cheaper = cheap
		return true
	}
	// An activation's offer reads the pool and the printed sources together
	// (the walk's activateOffers), not the pool alone: a folded-equal node whose
	// pool is spendable where the seen one's ghost unit was not (Woodland
	// Weavemaster's granted {G} after its Elf-only printed mana, CR 106.6)
	// still offers it, as gorge's node does.
	if g.d.hasPlay && g.d.play.Kind() == AkActivate && t.affordsUngrantedPlay(g.k, g.p, g.d.play, cost, g.ex, g.d) && g.witnessKeeps(t, cost) {
		g.cheaper = cheap
		return true
	}
	return false
}

// leafPays is a fresh grant-search node's success: gorge's offered check
// (offerPays, as seenPays does at a seen node), then the planner's price
// from the pool and the printed sources. cheap marks a repriced leaf.
func (g *grantSrch) leafPays(t *State, cost *kop.ManaCost, cheap bool) bool {
	if g.offerPays(t) {
		g.cheaper = true // the node's walk lists the play the root price refused
		return true
	}
	if g.ungrantedPays(t, cost) && g.witnessKeeps(t, cost) {
		g.cheaper = cheap
		return true
	}
	return false
}

// ungrantedPays is the planner's price of the play from the pool and the printed
// sources at node t. A spell's census leaves out a dynamic {T}, sacrifice source
// (Lotus Blossom: gorge's payment planner cannot price it, ungrCastCensus), so a
// cast it would complete is the search's gap step to float, not a priced leaf
// (mana_grantdynsac.go).
func (g *grantSrch) ungrantedPays(t *State, cost *kop.ManaCost) bool {
	if g.d.hasPlay && isCastKind(g.d.play.Kind()) && g.d.play.Kind() != AkActivate && g.k.t != nil && !t.fixedTapsCast(g.k, g.d.play) {
		return t.affordsUngrantedCast(g.k, g.p, cost, g.d)
	}
	return t.affordsUngrantedPlay(g.k, g.p, g.d.play, cost, g.ex, g.d)
}
