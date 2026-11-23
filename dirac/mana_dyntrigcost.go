package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// potentialDynOpen reports whether the trigger-cost offer gate
// (interp_ask.go trigCostPotentialPays) opens because p controls an untapped
// source with a mana ability gorge's potential walk prices potentialUnbounded
// (rules potential.go addPotentialMana) while a source on the board still
// yields mana now and the ability's production covers the ask's cost
// (dynCoversCost).
//
// gorge prices two shapes unbounded that the census prices at what they hold
// now: an indeterminate Amount$ in the production's faces (an X, a Count$
// expression, a Sacrificed$ reference: addIndeterminatePotentialMana, CR
// 107.4a) and an open production (Any, each-colour, an unknown token) in every
// colour (producedOpen over addPotentialMana). The census leaves both out of
// its planner view -- gen.go clears manaAb for a dynamic amount, and every
// non-varPC shape (a paid filter's {2}, a RestrictValid$ source, a
// commander-identity production, an each-colour gap) has no varPC entry -- so
// availPotentialFor's fold alone could not reach the cost: the ask was
// decline-only while gorge offered pay beside decline (CR 118.3: the payer
// elects pay, the charge fails, the payer declines). The varPC family (Lotus
// Blossom's {T}, SacSelf) was this arm's first case; the scan now keys on
// gorge's own condition, the ability's flags, not on the census view.
//
// The dynamic source must be one gorge's own gate admits, and the admission is
// the potential walk's, not the census's: dynSourceAdmits reads the walk's
// membership gates (potentialAdmits) without the census's zero-amount
// exclusions, and prices the activation's own {N} from the other sources
// (manaAbilityPayablePool). The over-bound's unbounded claim is per produced
// face, not per colour: a fixed-face production (Elvish Archdruid's "one G per
// counter") prices G unbounded and every other kind at zero, so a trigger's
// {2}{B}{B} stays decline-only on both engines while the generic alone opens
// the gate.
func (s *State) potentialDynOpen(k *prep, p uint8, cost *kop.ManaCost, d *payDesc) bool {
	if k.gen == nil || !s.anyLiveSource(k, p, d) {
		return false
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		cd := &k.t.Cards[s.cardAt(sl)]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if j >= 16 || a.Kind != kop.AbMana || a.AFlags&kop.AfLatent != 0 {
				continue
			}
			dynamic := a.AFlags&kop.AfManaDynamic != 0
			each := a.AFlags&kop.AfManaEachColor != 0
			if !dynamic && !each {
				continue
			}
			// a reflected Amount$ production has its own prepared views
			// (mana_reflamt.go): gorge's addPotentialReflected prices it at the
			// reflected colours, not unbounded, so it is no shape of this fold
			if a.AFlags&kop.AfManaReflect != 0 && !a.ManaIdentity {
				continue
			}
			// a counter-removal amount ("Remove X counters: Add X mana") is
			// priced at the counters it can remove when the payment reads the
			// sub-counter bound (rules potential_subcounter.go), not unbounded:
			// the trigger-cost gate's descriptor sets SubCounterBound, so a
			// storage battery's announced X stays bounded here
			if d != nil && d.subBound && a.ManaV.K == kop.VCountersSelf &&
				a.ManaV.N == int16(a.Cost.Ctr) && a.Cost.Flags&kop.CostLoyaltyX != 0 {
				continue
			}
			// a commander-identity production with no commander reads no mana
			// (gorge potentialIdentityEmpty, CR 903.4f): its indeterminate
			// amount's producedOpen fallback would otherwise price every colour
			if (a.ManaIdentity || a.ManaIdentityPotential) && s.identityColours(k, s.perms[sl].ctrl) == 0 {
				continue
			}
			if s.dynSourceAdmits(k, p, sl, j, a, d) && dynCoversCost(a, cost) {
				return true
			}
		}
	}
	return false
}

// dynCoversCost reports whether a dynamic-amount production covers the ask's
// cost under the over-bound's pricing, gorge's pool read for the trigger-cost
// offer gate (respay.go resPayNeedsMana over pay.CostPayablePool): the pool
// prices the indeterminate amount unbounded in exactly the produced faces
// (rules potential.go addIndeterminatePotentialMana, CR 107.4a -- the faces
// are the only colours the ability makes), so an open production ("mana of
// any one color", ColAny) is unbounded in every kind and covers any cost,
// while a fixed mask is unbounded in its faces alone and funds no pip outside
// them; the generic part any face funds, for every kind of mana pays it. An
// each-colour or identity production is open in every kind (producedOpen over
// addPotentialMana prices it there), and a blank production is the executor's
// colourless default.
func dynCoversCost(a *kop.Ability, cost *kop.ManaCost) bool {
	if cost == nil {
		return true
	}
	if cost.Snow != 0 {
		return false // the {S} bound: only the pool pays it (mana.go affords)
	}
	if a.AFlags&kop.AfManaEachColor != 0 || a.AFlags&kop.AfManaDynamic != 0 && (a.ManaIdentity || a.ManaIdentityPotential) {
		return true // open production: the whole-pool open, every kind unbounded
	}
	m := uint8(a.ManaColors)
	if m == 0 {
		m = uint8(kop.ColC) // a blank production is the executor's colourless default (potential.go addPotentialMana)
	}
	if m == uint8(kop.ColAny) {
		return true // open production: the whole-pool open, every kind unbounded
	}
	for i, n := range cost.Pips {
		if n > 0 && m&(1<<uint(i)) == 0 {
			return false // a pip the produced faces do not make
		}
	}
	for i, h := range cost.Hyb {
		if h == 0 || i == 0 && cost.Tb != 0 || h == kop.HybPhyrexian {
			continue // the twobrid marker pays as generic; a phyrexian pip stays the pool fold's
		}
		if uint8(h)&m == 0 {
			return false // neither of the pip's faces lies among the produced faces
		}
	}
	return true
}

// dynSourceAdmits is potentialAdmits without the census amount gates: the
// zero-counter exclusion (sacCtrManaAb, gorge manaAbilityYieldsNothing) and
// the dynamic-amount variant's manaAb-less view are the sources flag's strict
// read, which anyLiveSource carries; the potential walk's admission is the
// activation's payability (gorge manaAbilityPayablePool) -- whatever the
// amount resolves to now, the walk prices it potentialUnbounded. The
// activation's own {N} fee is read from the sources other than sl (a source's
// own tap cannot pay its own activation): a {2}, {T} filter joins only once
// the rest of the board funds the {2}.
func (s *State) dynSourceAdmits(k *prep, p uint8, sl Slot, j int, a *kop.Ability, d *payDesc) bool {
	pp := &s.perms[sl]
	if pp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.genLost(k, sl) {
		return false
	}
	if a.Cost.Flags&^(kop.CostTap|kop.CostSacSelf|kop.CostPayLife|kop.CostOncePerTurn|kop.CostOncePerGame) != 0 && !s.potentialGapCost(k, p, sl, j, a) {
		return false
	}
	if a.Cost.Flags&kop.CostOncePerTurn != 0 && j < 8 && pp.actUsed&(1<<j) != 0 {
		return false
	}
	if a.Cost.Flags&kop.CostOncePerGame != 0 && s.onceBlocked(k, sl, a) {
		return false
	}
	if a.Cost.Flags&kop.CostPayLife != 0 && int(s.pl[SeatIx(p)].life) < lifeCostOwed(&a.Cost) {
		return false
	}
	if k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	if a.Cost.Flags&kop.CostTap != 0 && (pp.flags&pfTapped != 0 || s.activationSick(k, sl)) {
		return false
	}
	if a.Trig.Cond != 0 && !s.condEval(k, a.Trig.Cond, evalCtx{src: pp.obj, srcSl: sl, ctrl: pp.ctrl}) {
		return false
	}
	// the activation's own cost is payable: a dynamicManaCost shape's only
	// cost beyond the tap is sacrificing the source, which has its candidate
	// while the source is on the battlefield (potentialGapCost's sacrifice arm)
	if a.Cost.Flags&kop.CostSacSelf != 0 && a.Cost.N > 0 &&
		s.countCostSac(k, s.perms[sl].ctrl, &a.Cost, sl) < int(a.Cost.N) {
		return false
	}
	if (a.Cost.Mana.MV() != 0 || a.Cost.Mana.X != 0) && !s.dynFeePayable(k, p, sl, &a.Cost.Mana) {
		return false
	}
	return d == nil || a.ManaRestrict == 0 || s.restrSrcAdmits(k, p, sl, a, d)
}

// dynFeePayable reports whether the sources other than sl, plus the floating
// pool, cover a mana ability's own {N} activation cost. It is gorge's
// manaAbilityPayablePool pricing for the fixpoint, read on the current board:
// a source's own production never funds its own activation (one tap cannot do
// both), and a source that makes nothing now funds nothing.
func (s *State) dynFeePayable(k *prep, p uint8, sl Slot, cost *kop.ManaCost) bool {
	var pot [kop.NumManaKinds]int
	for c := range pot {
		pot[c] = int(s.pl[SeatIx(p)].pool[c])
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		o := m.first()
		pc := s.manaPCGen(k, o)
		if o == sl || !s.canTapForManaPC(k, o, pc) {
			continue
		}
		mask, n := s.srcMaskAt(pc, o), s.srcYieldAt(k, o, pc)
		for c := 0; c < kop.NumManaKinds; c++ {
			if mask&(1<<uint(c)) != 0 {
				pot[c] += int(n)
			}
		}
	}
	return potentialCovers(&pot, cost)
}

// anyLiveSource is the sources flag gorge's resPayNeedsMana gates the
// trigger-cost offer on (rules respay.go, potential.go): a battlefield mana
// source whose abilities add at least one mana right now
// (untappedManaSourceFor, windowSourceYields). It is anyManaSource without
// the arms that count a source whose printed ability is admitted while it
// makes nothing now -- the zero-amount dynamic source, the each-colour gap
// whose count is zero -- so the dynamic-amount arm above does not ride them:
// beside one alone gorge's flag is false and the ask stays decline-only.
func (s *State) anyLiveSource(k *prep, p uint8, d *payDesc) bool {
	if k.gen == nil {
		return false
	}
	// a deferred source (Pyromancer's Goggles, mana_deferred.go) has no census
	// ability, yet its plain {R} is untapped mana gorge's untappedManaSourceFor
	// counts (CR 106.6: the when-spent rider restricts nothing)
	if s.scriptDeferred(k, p).any() {
		return true
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		pc := s.manaPCGen(k, sl)
		if s.canTapForManaPC(k, sl, pc) || k.sacOnlyMana && s.canSacOnlyForMana(k, sl, pc) {
			return true
		}
		if s.storeSourceCounts(k, p, sl) {
			return true // a storage battery holding counters (mana_storesource.go)
		}
		if s.riderPotential(k, p, sl) {
			return true // a bounded rider source's activation (CR 605.3b)
		}
		if s.identityPotentialSource(k, p, sl, d) {
			// a dynamic identity production makes nothing while its amount is
			// zero, so it is no source now (gorge windowSourceYields resolves
			// the amount strictly)
			if a, _, ok := s.identityPotentialAbility(k, sl); ok && d != nil && !s.dynSourceYields(k, sl, a) {
				continue
			}
			return true // a ColorIdentity source the payment-bound fold prices (CR 903.4)
		}
		c := s.cardAt(sl)
		// a printed planner mana ability (Springleaf Drum's tap-another, a
		// filter) is a source while its activation is admitted and it makes
		// something now: a dynamic amount that resolves to zero is no source
		// (gorge windowSourceYields' strict read), so the walk's zero-amount
		// admission is not the flag's
		if j := int(k.cards[c].manaAb); j >= 0 {
			// a first-only filter ({2}, {T}: Celestial Prism) is a source only
			// while the live pool pays its fee: gorge's untappedManaSourceFor
			// reads availableManaAbilities, whose payability gate prices the
			// floating mana alone, not the other sources' potential (CR 605.3a;
			// windowFiltFirstUsable is the window's read of the same gate)
			feeUnpaid := k.gen.has(CardID(c), gfFilt) && s.filtFirstOnly(k, sl) && !s.windowFiltFirstUsable(k, p, sl)
			// a commander-identity source whose controller has no commander
			// yields nothing now (CR 903.4f): its planner view is the
			// no-ability one (pc.manaAb < 0, gorge manaAbilityYieldsNothing),
			// so the card's printed unit must not count it as a source
			if ab := &k.t.Cards[c].Abilities[j]; !feeUnpaid && !kop.DiscardHandManaAb(ab) &&
				!(pc.manaAb < 0 && ab.ManaIdentity) &&
				s.potentialAdmits(k, p, sl, j, ab, nil) && s.srcYieldAt(k, sl, pc) > 0 {
				return true
			}
		}
		// an each-colour gap (Bloom Tender) is a census gap with no planner
		// manaAb: gorge's walk counts it while its count makes at least one
		// mana now, and potentialManaFor prices it open in every colour
		if j := k.gen.eachCol(c); j >= 0 && s.eachColorOut(k, p, sl) != 0 &&
			s.potentialAdmits(k, p, sl, int(j), &k.t.Cards[c].Abilities[j], d) {
			return true
		}
		// a tap-another ability beside a spent plain one (Relic of Legends, CR
		// 605.3a) and a counter-cost gap source (Wall of Roots, CR 605.1a)
		// yield while their permanent to tap, or counters, are there
		if k.tapXSecond(c) && s.potentialAdmits(k, p, sl, int(k.tapXAb[c]), &k.t.Cards[c].Abilities[k.tapXAb[c]], d) {
			return true
		}
		if ab := k.ctrAbOf(c); ab != nil {
			if kop.StoreManaAb(ab) {
				if s.perms[sl].ctrN(s, int(ab.Cost.Ctr)) > 0 && s.potentialAdmits(k, p, sl, int(k.ctrAb[c]), ab, d) {
					return true
				}
			} else if k.cards[c].manaAb < 0 && s.potentialAdmits(k, p, sl, int(k.ctrAb[c]), ab, d) {
				return true
			}
		}
		// a restricted ability the payment may spend (Queen's Diamond's
		// commander-only mana refused, Hydraulic Helper's admitted, CR 106.6).
		// A dynamic amount that resolves to zero now makes nothing, so it is
		// no source for the window flag however its restriction reads
		// (gorge windowSourceYields' strict amount read).
		cd := &k.t.Cards[c]
		for j := range cd.Abilities {
			if ab := &cd.Abilities[j]; j < 16 && ab.Kind == kop.AbMana && ab.AFlags&kop.AfLatent == 0 &&
				ab.ManaRestrict != 0 && (d == nil || s.restrSrcAdmits(k, p, sl, ab, d)) && s.potentialAdmits(k, p, sl, j, ab, nil) &&
				s.dynSourceYields(k, sl, ab) {
				return true
			}
		}
	}
	return false
}

// dynSourceYields reports whether a mana ability whose amount the census reads
// dynamically makes at least one mana now: gorge's windowSourceYields resolves
// a non-literal Amount$ strictly (rules payment_window_yield.go), so a
// source holding none is no source for the trigger-cost offer however its
// ability is admitted.
func (s *State) dynSourceYields(k *prep, sl Slot, a *kop.Ability) bool {
	if a.AFlags&kop.AfManaDynamic == 0 {
		return true
	}
	return s.dynManaN(k, a, sl) > 0
}
