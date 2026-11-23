package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// restrBoundDrops: on the any-mana walk (an unrelaxable census: every play
// gorge's potential walk lists is searched), play a is not one the walk lists.
// gorge prices each play against the potential bound built for that payment
// (potentialPoolFor, potentialRestrictionAdmits, CR 106.6): a restricted
// source whose RestrictValid$ refuses the spell adds nothing to it. The open
// walk counts every source's mana for every play, so a Jeweled Lotus (three
// mana of one colour, only to cast its owner's commander) opened the bound for
// an Ordeal of Thassa that Urza's once-counted tap-an-artifact mana could not
// reach. A bound that is closed once the payment's restrictions apply, and
// does not afford the cast, lists no such play.
func (s *State) restrBoundDrops(k *prep, p uint8, a Action) bool {
	if isCastKind(a.Kind()) && a.Mode() == 0 {
		return s.castBoundDrops(k, p, a)
	}
	if a.Kind() == AkActivate {
		return s.actBoundDrops(k, p, a)
	}
	return false
}

// castBoundDrops is restrBoundDrops' cast arm: play a is a plain cast the
// bound built for that payment cannot afford.
func (s *State) castBoundDrops(k *prep, p uint8, a Action) bool {
	var open avail
	if !s.availPotentialFor(k, p, &open, nil, false) {
		return false // the bound is closed for every payment: the gaps walk's plays
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	d := s.payDescNow(o)
	d.repeatBound = true
	var av avail
	if s.availPotentialFor(k, p, &av, &d, false) {
		return false // open for this payment too: an unrestricted open source
	}
	cost := s.adjustFit(k, &av, p, c, o, k.cards[c].cost, true)
	return !av.affords(&cost, pipMaskOf(&cost), 0)
}

// actBoundDrops is restrBoundDrops' arm for a battlefield activation, gorge's
// potential walk's third play kind (scriptOffers' potentialWalkOf): the same
// bound read over a plain printed cost. gorge prices an activation offer
// against the payment's bound (pay.AvailableFor, CR 106.6; legal_walk_battlefield
// autoPayOffer's PotentialMana), so Jeweled Lotus's commander-only mana is not
// in the bound an Urza, Lord High Artificer {5} shuffle is priced against,
// while the open any-mana walk's 99 units pay it (tail-64 pair 137, decision
// 575). An X, hybrid, snow, Phyrexian, own-reduce, Power-up, waterbend, tap-X
// or sacrifice cost keeps its own gate (activateOffers' pool-only arm, the
// hand activations' handActCensusPays), as does a granted one (index 255).
func (s *State) actBoundDrops(k *prep, p uint8, a Action) bool {
	if a.Mode() == 255 {
		return false // a granted or gained ability: no printed cost to price
	}
	o := Obj(a.Source())
	if o >= MaxObj {
		return false
	}
	sl := s.objs[o].slot
	if sl == NoSlot {
		return false // a hand or top-of-library activation: handActCensusPays' census
	}
	cid := s.cardAt(sl)
	abs := k.t.Cards[cid].Abilities
	if int(a.Mode()) >= len(abs) {
		return false
	}
	ab := &abs[a.Mode()]
	if ab.Kind != kop.AbActivated || !plainPricedCost(ab) {
		return false
	}
	mana := ab.Cost.Mana
	if k.gen != nil && k.gen.anyAbReduce {
		mana = s.abReduced(k, ab, sl, s.perms[sl].ctrl, mana) // the offer priced the reduced cost (canActivate)
	}
	if !sourcesPay(&mana) {
		return false
	}
	var open avail
	if !s.availPotentialFor(k, p, &open, nil, false) {
		return false // the bound is closed for every payment: the gaps walk's plays
	}
	var av avail
	d := payDesc{class: pdAct, card: cid, src: sl, repeatBound: true}
	if s.availPotentialFor(k, p, &av, &d, false) {
		return false // open for this payment too: an unrestricted open source
	}
	pm := pipMaskOf(&mana)
	if ab.Cost.Flags&kop.CostTap != 0 && s.canTapForMana(k, sl) {
		// the source's own tap cannot fund its cost's mana (abilityAffords)
		var av2 avail
		av2 = av
		if !s.ownTapBoundKept(k, sl) {
			pc := &k.cards[cid]
			if k.gen != nil {
				pc = s.manaPCGen(k, sl)
			}
			s.removeSrc(k, &av2, sl, pc)
		}
		return !av2.affords(&mana, pm, 0)
	}
	return !av.affords(&mana, pm, 0)
}
