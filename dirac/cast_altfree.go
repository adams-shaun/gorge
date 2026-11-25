package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// prepAltFree records each card's own alternative cost: a
// StatAlternativeCost static whose Cost is mana only (Land Grant's "reveal
// your hand rather than pay this spell's mana cost", the zero cost: kgen
// lowers Reveal<1/Hand> as no cost, gorge cost.go IsWholeHandRevealSpec;
// Blasphemous Edict's {B}). A card with an either-or additional cost keeps
// none (build.altAdd carries the alternative-cost cast).
func prepAltFree(t *kop.Table, g *genPrep, p *prep) {
	for i := range t.Cards {
		if t.Cards[i].HasAltAdd {
			continue
		}
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost && (a.Cost.Flags == 0 && a.Cost.N == 0 || altPickCost(&a.Cost)) && a.Cost.Mana.X == 0 && j < 255 {
				if g.altFree == nil {
					g.altFree = make([]uint8, len(t.Cards))
				}
				g.altFree[i] = uint8(j + 1)
				p.cards[i].altCost = true
				break
			}
		}
	}
}

// altSacCost: the alternative cost is sacrificing N permanents matching its
// filter and nothing else (Fireblast's Sac<2/Mountain>; kgen altSacOK keeps
// it to an instant or sorcery with no other cost).
func altSacCost(c *kop.Cost) bool {
	// PayLife rides Cost.Ctr beside the sacrifice's Cost.N (Wickerfolk
	// Indomitable, Demon of Death's Gate); the mana may be nonzero (a
	// Borderpost's {1} plus a Return, Sephara's {W} plus taps)
	return c.Flags&^(kop.CostSac|kop.CostPayLife) == 0 && c.Flags&kop.CostSac != 0 && c.N > 0
}

// altReturnCost: the alternative cost is returning N permanents matching its
// filter to their owners' hands and nothing else (Gush's Return<2/Island>,
// Thwart's Return<3/Island>, Daze's Return<1/Island>; gorge cost.go
// returnCost, the pick posed as returnAsk; kgen altSacOK keeps it to an
// instant or sorcery with no other cost).
func altReturnCost(c *kop.Cost) bool {
	return c.Flags == kop.CostReturn && c.N > 0
}

// altHandExileCost: the alternative cost is exiling N hand cards matching its
// filter, and paying N life beside it or not, nothing else (Force of Will's
// PayLife<1> and ExileFromHand<1/Card.Blue+Other>, Misdirection's exile alone,
// the Force cycle's bare ExileFromHand<1/Card.Blue+Other>, CR 118.9; kgen
// altPayLifeExileHand).
func altHandExileCost(c *kop.Cost) bool {
	shared := c.Flags == kop.CostPayLife|kop.CostExileGrave
	bare := c.Flags == kop.CostExileGrave
	return (shared || bare) && c.ExileHand && c.N > 0
}

// altDiscardCost: the alternative cost is one discard part, or two, and
// nothing else (Abolish's Discard<1/Plains>; Foil's Discard<1/Island> then
// Discard<1/Card.Other>, CR 118.3; kgen altDiscardShape): the CostDiscard
// part, filtered or not, then Cost.Discard2N cards when a second part is
// owed (cost_discardfiltered.go).
func altDiscardCost(c *kop.Cost) bool {
	return c.Flags&^kop.CostDiscardFiltered == kop.CostDiscard && c.N > 0 &&
		!c.DiscardHand
}

// altLifeCost: the alternative cost is paying N life and nothing else (Snuff
// Out's PayLife<4>, CR 118.9, 119.4; kgen altLifeOrTapShape): no pick is asked,
// the offer is gated on the life and the cast pays it (payLifeCost).
func altLifeCost(c *kop.Cost) bool {
	return c.Flags == kop.CostPayLife && c.N > 0
}

// altTapCost: the alternative cost is tapping N untapped permanents matching
// its filter and nothing else (Ramosian Rally's tapXType<1/Creature>, CR
// 118.9, 602.2b; gorge cost.go tapPermanentCostAsk, kgen altLifeOrTapShape):
// the cast poses the tap picks (costPickOptions' bsTapCost).
func altTapCost(c *kop.Cost) bool {
	return c.Flags == kop.CostTapX && c.N > 0
}

// altPickCost: the alternative cost's non-mana parts are picks the cast asks
// (the sacrifice, the return, the hand exile, the two discards or the taps) or
// life it pays, rather than a bare mana price.
func altPickCost(c *kop.Cost) bool {
	return altSacCost(c) || altReturnCost(c) || altHandExileCost(c) || altDiscardCost(c) || altLifeCost(c) || altTapCost(c)
}

// offerAltFreeHand offers hand card o's cast for its own free alternative
// cost (gorge rules alternativeCosts' face walk, legal_walk_hand.go: an
// AltCostIndex 1 "cast" option) while the static's condition holds for the
// caster (gorge alternativeCostScopeOK's Condition$ and CheckSVar$; Land
// Grant: no land card in hand, the Force cycle: not the caster's turn). It
// rides the hand free-cast mode castMayPlay|castFree,
// which the oracle canonizes as the AltCostIndex 1 cast; castFlagsGen spends
// no permission for it. The caller has checked timing and cast prohibition.
//
//go:noinline
func (s *State) offerAltFreeHand(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	a := &k.t.Cards[c].Abilities[k.gen.altFree[c]-1]
	if a.Static.Cond2 != 0 && !s.condEval(k, a.Static.Cond2, evalCtx{src: o, srcSl: NoSlot, ctrl: p}) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || s.castHeldOut(o) {
		return
	}
	if altPickCost(&a.Cost) {
		// the sacrifice alternative (Fireblast), the return one (Gush), or
		// the pay-life and hand-exile one (Force of Will): gorge's hand walk
		// offers its AltCostIndex 1 cast while the permanents can be
		// sacrificed or returned / the life paid and the cards exiled; the cast asks the targets, then
		// the picks (CR 601.2b-h; buildPickCost serves the static's cost)
		ac := s.adjustCost(k, p, c, o, a.Cost.Mana)
		if !s.pickCostPayable(k, p, &a.Cost, o) || !av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false) {
			return
		}
		d.add(MakeAction(AkCast, TCode(o), castAltTgt, 0))
		return
	}
	if a.Cost.Mana == (kop.ManaCost{}) {
		if s.freeOwes(k) {
			// the free cast still pays the cost statics' increases (CR 601.2f,
			// 118.9d: Thalia's {1} on Refreshing Rain). The planner surface
			// skips AltCostIndex options, so it is offered only while the
			// floating pool pays what it owes, as the mana alternative below
			if ac := s.adjustCost(k, p, c, o, a.Cost.Mana); ac != (kop.ManaCost{}) && (!av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false)) {
				return
			}
		}
		d.add(MakeAction(AkCast, TCode(o), 0, castMayPlay|castFree))
		return
	}
	// a mana alternative (Blasphemous Edict's {B}): the cost modifiers
	// apply to it (CR 601.2f). Gorge's hand walk offers it against
	// untapped sources (legal_walk_hand.go), but the oracle plays the
	// payment-planner surface, which skips AltCostIndex options
	// (payment_plan.go): offered only when the floating pool pays it, the
	// planner-surface rule of offspring and kicker (altcost lockstep)
	ac := s.adjustCost(k, p, c, o, a.Cost.Mana)
	if !av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false) {
		return
	}
	d.add(MakeAction(AkCast, TCode(o), castAltTgt, 0))
}

// castAltTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's own mana alternative cost (prepAltFree), the
// oracle's AltCostIndex 1 cast; legal.go sets genState.altCast for
// beginCast, which marks build.altAdd = buildAltCost.
const castAltTgt = 1

// buildAltCost in build.altAdd: the spell is cast for its own alternative
// mana cost (buildCostGen prices it).
const buildAltCost = 2

// takeAltCast marks the build beginCast starts with the alternative cost
// genState.altCast names (buildAltCost, or kw_beammeup.go's buildBeamGrant)
// and clears the flags.
func (s *State) takeAltCast(k *prep, c CardID, mode uint8) {
	if k.gen == nil || !s.g.altCast {
		return
	}
	s.g.altCast = false
	if mode&2 == 0 {
		s.bld().altAdd = buildAltCost
	} else if !k.t.Cards[c].BeamMeUp || s.g.beamGrant {
		s.bld().altAdd = buildBeamGrant // kw_beammeup.go
	}
	s.g.beamGrant = false
}

// altCostOf is the adjusted alternative mana cost of the spell being built.
func (s *State) altCostOf(k *prep) kop.ManaCost {
	e := &s.bld().e
	a := &k.t.Cards[e.card].Abilities[k.gen.altFree[e.card]-1]
	return s.adjustCostGen(k, e.ctrl, e.card, e.obj, a.Cost.Mana)
}

// altBaseMana is the mana of the alternative cost the build pays, before the
// cost statics (altCostOf adjusts it).
func (s *State) altBaseMana(k *prep) kop.ManaCost {
	e := &s.bld().e
	return k.t.Cards[e.card].Abilities[k.gen.altFree[e.card]-1].Cost.Mana
}

// altFreeCast reports whether a castMayPlay|castFree cast of o is its own
// free alternative cost from the hand (offerAltFreeHand), not a play
// permission's.
func (s *State) altFreeCast(k *prep, o Obj, mode uint8) bool {
	c := s.objs[o].card
	return mode&(castMayPlay|castFree) == castMayPlay|castFree && k.gen.altFree[c] != 0 && s.objs[o].zone == kop.ZoneHand
}
