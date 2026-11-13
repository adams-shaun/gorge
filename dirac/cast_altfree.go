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
	return c.Flags == kop.CostSac && c.N > 0 && c.Mana == (kop.ManaCost{})
}

// altHandExileCost: the alternative cost is exiling N hand cards matching its
// filter, and paying N life beside it or not, nothing else (Force of Will's
// PayLife<1> and ExileFromHand<1/Card.Blue+Other>, Misdirection's exile alone,
// the Force cycle's bare ExileFromHand<1/Card.Blue+Other>, CR 118.9; kgen
// altPayLifeExileHand).
func altHandExileCost(c *kop.Cost) bool {
	shared := c.Flags == kop.CostPayLife|kop.CostExileGrave
	bare := c.Flags == kop.CostExileGrave
	return (shared || bare) && c.ExileHand && c.N > 0 && c.Mana == (kop.ManaCost{})
}

// altDiscardTwoCost: the alternative cost is two discard parts and nothing
// else (Foil's Discard<1/Island> then Discard<1/Card.Other>, CR 118.3; kgen
// altDiscardTwoShape): the CostDiscard part, filtered or not, then
// Cost.Discard2N cards (cost_discardfiltered.go).
func altDiscardTwoCost(c *kop.Cost) bool {
	return c.Flags&^kop.CostDiscardFiltered == kop.CostDiscard && c.N > 0 && c.Discard2N > 0 &&
		!c.DiscardHand && c.Mana == (kop.ManaCost{})
}

// altPickCost: the alternative cost's non-mana parts are picks the cast asks
// (the sacrifice, the hand exile or the two discards) rather than a bare
// mana price.
func altPickCost(c *kop.Cost) bool {
	return altSacCost(c) || altHandExileCost(c) || altDiscardTwoCost(c)
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
		// the sacrifice alternative (Fireblast), or the pay-life and
		// hand-exile one (Force of Will): gorge's hand walk offers its
		// AltCostIndex 1 cast while the permanents can be sacrificed / the
		// life paid and the cards exiled; the cast asks the targets, then
		// the picks (CR 601.2b-h; buildPickCost serves the static's cost)
		ac := s.adjustCost(k, p, c, o, a.Cost.Mana)
		if !s.pickCostPayable(k, p, &a.Cost, o) || !av.affords(&ac, pipMaskOf(&ac), 0) || !s.windowPoolPays(p, &ac, false) {
			return
		}
		d.add(MakeAction(AkCast, uint8(o), castAltTgt, 0))
		return
	}
	if a.Cost.Mana == (kop.ManaCost{}) {
		d.add(MakeAction(AkCast, uint8(o), 0, castMayPlay|castFree))
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
	d.add(MakeAction(AkCast, uint8(o), castAltTgt, 0))
}

// castAltTgt in an AkCast's target byte (mode 0, from the hand) is the
// cast for the card's own mana alternative cost (prepAltFree), the
// oracle's AltCostIndex 1 cast; legal.go sets genState.altCast for
// beginCast, which marks build.altAdd = buildAltCost.
const castAltTgt = 1

// buildAltCost in build.altAdd: the spell is cast for its own alternative
// mana cost (buildCostGen prices it).
const buildAltCost = 2

// altCostOf is the adjusted alternative mana cost of the spell being built.
func (s *State) altCostOf(k *prep) kop.ManaCost {
	e := &s.bld().e
	a := &k.t.Cards[e.card].Abilities[k.gen.altFree[e.card]-1]
	return s.adjustCostGen(k, e.ctrl, e.card, e.obj, a.Cost.Mana)
}

// altFreeCast reports whether a castMayPlay|castFree cast of o is its own
// free alternative cost from the hand (offerAltFreeHand), not a play
// permission's.
func (s *State) altFreeCast(k *prep, o Obj, mode uint8) bool {
	c := s.objs[o].card
	return mode&(castMayPlay|castFree) == castMayPlay|castFree && k.gen.altFree[c] != 0 && s.objs[o].zone == kop.ZoneHand
}
