package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Casting a commander from the command zone (CR 903.8). A Commander game
// offers the seat's own commander while it is in the command zone, at the
// card's timing and with the card's plain-cast gates; the cost is the
// card's mana cost plus the tax of {2} for each earlier cast from the
// command zone, with every cost static applied to that total: the tax is an
// additional cost, part of the total cost a reduction takes generic off and
// a floor measures (CR 601.2f; gorge pay.CostMods.ApplyTaxedTo).

// offerCommanderCast offers each of p's commanders from the command zone
// when it is there and the hand cast of the same card would be legal. A
// partner pair is offered in command-zone order, as gorge walks its zone
// list: the commander that arrived last comes second.
func (s *State) offerCommanderCast(k *prep, d *Decision, p uint8, av *avail) {
	first, second := s.pl[p].cmd, s.cmdObj(p, 1)
	if !s.partnerLast(p) {
		first, second = second, first
	}
	s.offerCommanderSlot(k, d, p, av, first)
	s.offerCommanderSlot(k, d, p, av, second)
}

// offerCommanderSlot offers commander o (NoObj none) from the command zone.
func (s *State) offerCommanderSlot(k *prep, d *Decision, p uint8, av *avail, o Obj) {
	if o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		return
	}
	c := s.objs[o].card
	pc := &k.cards[c]
	if !pc.instant && !s.sorcerySpeed(p) && !s.castFlash(k, p, c) {
		return
	}
	if k.gen != nil && k.gen.anyCantCast && s.castForbidden(k, p, c, o) {
		return
	}
	cost := pc.cost
	addCommanderTax(&cost, s.commanderTax(p, o))
	if s.statM.any() || k.gen != nil {
		// the same reduction the hand walk composes (adjustFit), over the
		// taxed total
		av.delve = s.tbDelveCredit(k, p, c, o, true)
		cost = s.adjustFit(k, av, p, c, o, cost, false)
	}
	if !s.castAffordsR(k, p, c, s.restrOriginOfObj(o), &cost, av) ||
		!s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) {
		return
	}
	if s.castHeldOut(o) || !s.gorgeOffers(p, o, 0) {
		return
	}
	if cost.X != 0 && !s.spellXBoundPayable(k, c, p, &cost) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), 0, 0))
}

// commanderTax is the CR 903.8 generic tax on o's cast by p: two per
// earlier cast of that same commander from the command zone (a partner pair
// pays two separate taxes), and only for p's commander cast from there. The build's recorded origin answers once the spell is pushed; before
// that the object's zone is the origin.
func (s *State) commanderTax(p uint8, o Obj) uint8 {
	j := s.cmdSlot(p, o)
	if j < 0 || *s.cmdCastAt(p, j) == 0 {
		return 0
	}
	z := s.objs[o].zone
	if s.buildLive() && s.bld().bfl&bflPushed != 0 {
		z = s.bld().origin
	}
	if z != kop.ZoneCommand {
		return 0
	}
	t := 2 * int(*s.cmdCastAt(p, j))
	if t > 255 {
		t = 255
	}
	return uint8(t)
}

// addCommanderTax adds tax to a cost's generic, saturating at 255.
func addCommanderTax(c *kop.ManaCost, tax uint8) {
	if tax == 0 {
		return
	}
	g := int(c.Generic) + int(tax)
	if g > 255 {
		g = 255
	}
	c.Generic = uint8(g)
}
