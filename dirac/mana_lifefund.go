package dirac

import (
	"math/bits"
)

// lifeFundAt: sl is a covered life-only mana source of p (Treasonous Ogre's
// "Pay 3 life: Add {R}", prepSoleLifeMana) that gorge's prefix search
// activates as a funding source: it is no census gap, but its activation is
// listed at every priority node and costs no {T}, so the search takes it as
// often as the funding allows while the payer has the life (CR 605.1a). The
// tapped, summoning-sick and used sources all qualify; only a lost ability or
// a can't-be-activated static takes it out (lifeGapMask's gate). An any-colour
// production asks a colour the step does not branch, so it stays out.
func (s *State) lifeFundAt(k *prep, p uint8, sl Slot) bool {
	if !k.painMana || s.pickedForMana(s.perms[sl].obj) {
		return false
	}
	pc := s.manaPCGen(k, sl)
	if !lifeOnlyPain(pc) || bits.OnesCount8(uint8(pc.painCol)&0x1f) != 1 || uint8(pc.painCol)&^0x1f != 0 {
		return false
	}
	if s.genLost(k, sl) || k.gen != nil && k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	return int(s.pl[SeatIx(p)].life) >= int(pc.painN)
}

// fundLifeOnly is a life-only funding source's step: its life is paid and its
// mana floats. The source is not tapped and spends no use, so it is
// repeatable while the payer has the life (try's lethal guard ends a line the
// payment kills, as gorge's scriptSearch.visit does).
func (s *State) fundLifeOnly(k *prep, p uint8, sl Slot) {
	pc := s.manaPCGen(k, sl)
	col := bits.TrailingZeros8(uint8(pc.painCol))
	a := &k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb]
	s.payLifeCost(k, p, &a.Cost, 0)
	s.pl[SeatIx(p)].pool[col] += max(pc.manaN, 1)
	s.artTag(k, p, sl, col)
}

// lifeFund takes life-only funding source sl's step at a node of the filter
// search. An activation's prefix is the payment window the autopay offer
// prices with the sources beside the floating pool, so its node counts as a
// gap's does; a cast's node is the plain offer list's, which a floating pool
// pays alone (lifeFundedPays), so the path stays a funding one (gorge's
// scriptSearch.visit prices only a gap's or a funded-by-gap node with the
// planner).
func (fs *filtSearch) lifeFund(k *prep, t *State, sl Slot, funded int, gapUsed bool) bool {
	fs.lifeSteps++
	defer func() { fs.lifeSteps-- }()
	return fs.try(k, t, filtStep{sl: sl, lifeFund: true}, funded+1, gapUsed || isAbilityPlay(fs.a))
}

// lifeFundedPays reports whether the search's play is offered at t after
// life-only funding steps with no gap used: gorge lists a plain cast at the
// floated pool's own price (the sources beside it are not counted once mana
// floats), so the offer is the pool paying the cast's mana cost.
func (fs *filtSearch) lifeFundedPays(k *prep, t *State) bool {
	if fs.lifeSteps == 0 || fs.a.Kind() != AkCast || fs.a.Mode() != 0 {
		return false
	}
	m, ok := t.plannedCastCost(k, fs.p, Obj(fs.a.Source()), 0)
	return ok && m.X == 0 && m.XPip == 0 && t.poolPays(fs.p, &m) && fs.offers(k, t)
}
