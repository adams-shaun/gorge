package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the cold halves of three cost-static shapes
// (all copied from gorge, rules/statics_costapply.go and pay/costmods.go).
//
//   - StReduceColor (Brush Off, Color$ 1 U): the coloured pips come off the
//     cost first, the shortfall spills to generic (CostMods.ApplyTo).
//   - StBargained (Hamlet Glutton, Ice Out, Johann's Stopgap): the static
//     prices only the bargained cast, at the offer and at the charge.
//   - A RaiseCost with a ValidTarget$ (Dragon's Prey): applies at the charge
//     only, once a chosen target matches; the offer's potential pass skips
//     it (statics_costmods.go) and the target ask drops a candidate whose
//     raised cost the caster cannot pay (cast_pricing.go).
//
// No State: the flags are table data, k.bargainRed is prep data, and the
// pricing reads the build's own cas, flags and targets.

// DebugReduceSpellFlip is the mutation hook (TestReducespellMutation): it
// drops the Bargain discount, the colour reduction and the target raise at
// once.
var DebugReduceSpellFlip bool

// reduceColorTake takes a StReduceColor static's coloured counts (Static.B,
// kop.ReduceColorBits each, W U B R G C from bit 0) off base's pips and
// returns the shortfall, which spills to generic. Hybrid and Phyrexian pips
// are not coloured pips here and stay.
func reduceColorTake(base *kop.ManaCost, b int16) int {
	if DebugReduceSpellFlip {
		return 0
	}
	spill := 0
	for i := 0; i < kop.NumManaKinds; i++ {
		want := int(b>>(kop.ReduceColorBits*i)) & (1<<kop.ReduceColorBits - 1)
		take := min(want, int(base.Pips[i]))
		base.Pips[i] -= uint8(take)
		spill += want - take
	}
	return spill
}

// bargainCharge: the cost of card c (object o) is being charged for its
// bargained cast, the one gorge's "bargained" mode prices with the
// StBargained discount (CR 702.166). The election degrades to the plain cast
// when no sacrifice candidate remains (bargainOptions clears cas).
func (s *State) bargainCharge(k *prep, c CardID) bool {
	if s.ctl != ctlBuild || s.bld().e.card != c || s.bld().e.kind != seSpell {
		return false
	}
	if !k.t.Cards[c].Bargain {
		return s.optPaidCharge(k, c)
	}
	return s.bld().cas != 0 && s.bld().cas < casConspire
}

// optPaidCharge: the spell being built is the optional-cost cast of an
// OptionalCost card (the kicked cast, cost_optional.go). A self ReduceCost
// reading the optional cost's paid arm (Bite Down on Crime) rides the
// StBargained flag and prices only that cast, as gorge's costAmountCtx seeds
// the election for the optional-cost variant alone. A card never has a
// Bargain and an OptionalCost part (kgen refuses it).
func (s *State) optPaidCharge(k *prep, c CardID) bool {
	return s.bld().e.flags&sfKicked != 0 && k.gen != nil && k.gen.anyOptCost && optCostCard(k, c)
}

// offerOptCostOnly offers the optional-cost cast of o when the plain cast is
// unaffordable but the cast priced with the paid-arm reduction is: the
// bargained cast's rule (offerBargainOnly), for the OptionalCost card.
func (s *State) offerOptCostOnly(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if k.gen == nil || !k.gen.anyOptCost || !optCostCard(k, c) || s.rbAny(p) {
		return
	}
	cost := s.bargainedPrice(k, p, c, o, k.cards[c].cost)
	if !av.affords(&cost, pipMaskOf(&cost), 0) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &cost, av) || s.castHeldOut(o) {
		return
	}
	if s.optCostOffer(k, p, c, o) {
		d.add(MakeAction(AkCast, uint8(o), 0, 1))
	}
}

// raiseTargetOK: a RaiseCost static with a ValidTarget$ (Dragon's Prey)
// applies to the spell under construction once a chosen target matches f
// (gorge costTargetsMatch: any chosen target; a nil slice, the offer, never
// matches). The kernel's offer prices no build, so it never applies.
func (s *State) raiseTargetOK(k *prep, f kop.FilterID, p uint8, c CardID) bool {
	if DebugReduceSpellFlip || s.ctl != ctlBuild {
		return false
	}
	e := &s.bld().e
	if e.card != c || e.kind != seSpell {
		return false
	}
	for i := 0; i < int(e.nt); i++ {
		if s.costTargetMatch(k, f, p, e.tgt[i]) {
			return true
		}
	}
	return false
}

// raiseDropTargets drops from the target menu the candidates whose
// target-dependent raise (StTargetsFilter2 on a StatIncreaseCost) makes the
// spell unaffordable, each tried as the sole selection (gorge
// affordableTargetCandidates). A spell with no such raise keeps all n.
func (s *State) raiseDropTargets(k *prep, buf *[MaxOptions]uint8, n int) int {
	return s.dropTargetsOwing(k, buf, n, false)
}

// freeDropTargets is raiseDropTargets for a spell cast without paying its
// mana cost: the raise lands on the empty cost.
func (s *State) freeDropTargets(k *prep, buf *[MaxOptions]uint8, n int) int {
	return s.dropTargetsOwing(k, buf, n, true)
}

func (s *State) dropTargetsOwing(k *prep, buf *[MaxOptions]uint8, n int, free bool) int {
	e := &s.bld().e
	defer s.unmarkCostComposition(s.markCostComposition())
	cd := &k.t.Cards[e.card]
	has := false
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.StFlags&kop.StTargetsFilter2 != 0 {
			has = true
			break
		}
	}
	if !has {
		return n
	}
	var av avail
	s.availForBuilt(k, e, &av)
	nt, t0 := e.nt, e.tgt[0]
	m := 0
	for i := 0; i < n; i++ {
		e.nt, e.tgt[0] = 1, buf[i]
		base := cd.Cost
		if free {
			base = kop.ManaCost{}
		}
		cost := s.adjustCostGen(k, e.ctrl, e.card, e.obj, base)
		if e.flags&sfAnyMana != 0 {
			cost = anyType(cost)
		}
		if e.flags&sfKicked != 0 && !free {
			cost = addCost(cost, cd.Kicker)
		}
		cost.Generic = uint8(int(cost.Generic) + cost.XGeneric(int(e.x)))
		if av.affords(&cost, pipMaskOf(&cost), 0) {
			buf[m] = buf[i]
			m++
		}
	}
	e.nt, e.tgt[0] = nt, t0
	return m
}

// bargainedPrice is card c's cost priced as the bargained cast: gorge's
// offer composes the "bargained" scope, so the StBargained statics apply
// (spellConstraintMatchesBargain).
func (s *State) bargainedPrice(k *prep, p uint8, c CardID, o Obj, base kop.ManaCost) kop.ManaCost {
	return s.adjustCostGenB(k, p, c, o, o, base, true)
}

// bargainOffers is casualtyOffers for the bargained cast: gorge's hand walk
// offers the mode when the floating pool pays the cost the bargained scope
// composes, discount included. Without a StBargained static the price is the
// plain one and casualtyOffers answers.
func (s *State) bargainOffers(k *prep, p uint8, o Obj) bool {
	if !k.bargainRed {
		return s.casualtyOffers(p, o)
	}
	if s.cfg&(cfgGorgeLowering|cfgGorgeAutopay) == 0 {
		return true
	}
	return s.FeatPoolPays(p, o, 4) // mode bit 2: the bargained price
}

// offerBargainOnly offers the bargained cast of o when the plain cast is
// unaffordable but the discounted one is: gorge's hand walk prices the
// bargained mode under its own scope, so a card the discount alone makes
// payable is still offered (Hamlet Glutton with {3}{G}{G} floating). The
// gates are the ones the plain cast ran, the price is the bargained one. A
// restricted source on the battlefield or in the pool of cards (k.restrSrc)
// only adds payers, so the plain avail that affords the discount affords it
// with them too.
func (s *State) offerBargainOnly(k *prep, d *Decision, p uint8, o Obj, c CardID, av *avail) {
	if k.gen == nil || !k.t.Cards[c].Bargain || s.rbAny(p) || !s.bargainOffer(k, p, c) {
		return
	}
	cost := s.bargainedPrice(k, p, c, o, k.cards[c].cost)
	if !av.affords(&cost, pipMaskOf(&cost), 0) {
		return
	}
	if !s.spellTargetsFeasible(k, c, p) || !s.spellCostsFeasible(k, c, p, o) || !s.altAddCastable(k, c, p, o, &cost, av) || s.castHeldOut(o) {
		return
	}
	if s.bargainOffers(k, p, o) {
		d.add(MakeAction(AkCastCasualty, uint8(o), 0, 0))
	}
}

// tgtDepCost: a is a cost static whose application waits for the chosen
// target (a ReduceCost or RaiseCost with a ValidTarget$). gorge's cast
// planner declines such a cast's witness ("shape:target_dependent_cost",
// paymentPlanTargetDependentFor reads the RaiseCost group too), and its
// potential planner prices one from the composed cost (gorgeLowers,
// witnessAtCheapest).
func tgtDepCost(a *kop.Ability) bool {
	if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIncreaseCost && a.Static.StFlags&kop.StRaisePerTarget != 0 {
		return true // priced by the targets chosen: "more for each target beyond the first"
	}
	return a.Kind == kop.AbStatic && (a.Static.Kind == kop.StatReduceCost || a.Static.Kind == kop.StatIncreaseCost) &&
		a.Static.StFlags&kop.StTargetsFilter2 != 0 && a.Static.StFlags&kop.StRaiseLife == 0 // (a life raise prices no mana: cost_raiselife.go)
}

// cardTgtDepCost: cd carries such a static.
func cardTgtDepCost(cd *kop.CardDef) bool {
	for i := range cd.Abilities {
		if tgtDepCost(&cd.Abilities[i]) {
			return true
		}
	}
	return false
}
