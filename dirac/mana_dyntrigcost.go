package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// potentialDynOpen reports whether the trigger-cost offer gate
// (interp_ask.go trigCostPotentialPays) opens because p controls an untapped
// census source whose mana ability's amount is dynamic (AfManaDynamic, the
// gfManaVar family: Lotus Blossom's "Add X mana of any one color, where X is
// the number of petal counters on Lotus Blossom") while a source on the board
// still yields mana now and the ability's production covers the ask's cost
// (dynCoversCost). gorge's potential walk (rules potential.go
// addPotentialMana) prices such an amount potentialUnbounded in the
// production's faces -- an indeterminate Amount$ is never priced at zero,
// whatever it reads now -- while the census prices the amount the source
// holds (gen.go varPC: zero petals is zero mana and no planner unit,
// manaPCGen), so the gate's fold (availPotentialFor) could not reach the cost
// and the ask was decline-only while gorge offered pay beside decline (CR
// 118.3: the payer elects pay, the charge fails, the payer declines). The
// over-bound's unbounded claim is per produced face, not per colour, and
// gorge's gate still requires the potential pool to cover the cost's pips
// (respay.go resPayNeedsMana over pay.CostPayablePool): a fixed-face
// production (Elvish Archdruid's "one G per counter") prices G unbounded and
// every other kind at zero, so a trigger's {2}{B}{B} stays decline-only on
// both engines while the generic alone opens the gate.
//
// The dynamic source must be one gorge's own gate admits, and the admission is
// the potential walk's, not the census's: dynSourceAdmits reads the walk's
// membership gates (potentialRestrictionAdmits, potential.go) and the
// activation's payability (manaAbilityPayablePool) without the census's
// zero-amount exclusions.
func (s *State) potentialDynOpen(k *prep, p uint8, cost *kop.ManaCost, d *payDesc) bool {
	if k.gen == nil || !s.anyLiveSource(k, p, d) {
		return false
	}
	for m := s.ctrlM[SeatIx(p)]; m.any(); m = m.rest() {
		sl := m.first()
		c := s.cardAt(sl)
		if k.gen.flags[c]&gfManaVar == 0 {
			continue
		}
		cd := &k.t.Cards[c]
		for j := range cd.Abilities {
			a := &cd.Abilities[j]
			if a.Kind != kop.AbMana || a.AFlags&(kop.AfManaDynamic|kop.AfLatent) != kop.AfManaDynamic {
				continue
			}
			// a commander-identity production adds nothing unbounded to gorge's
			// pool: addPotentialIdentityColours declines an indeterminate
			// amount and the strip fallback adds nothing (potential.go), so
			// the shape stays decline-only here
			if a.ManaIdentity {
				continue
			}
			if dynamicManaCost(a) && s.dynSourceAdmits(k, p, sl, j, a, d) && dynCoversCost(a, cost) {
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
// them; the generic part any face funds, for every kind of mana pays it.
func dynCoversCost(a *kop.Ability, cost *kop.ManaCost) bool {
	if cost == nil {
		return true
	}
	if cost.Snow != 0 {
		return false // the {S} bound: only the pool pays it (mana.go affords)
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
// amount resolves to now, the walk prices it potentialUnbounded.
func (s *State) dynSourceAdmits(k *prep, p uint8, sl Slot, j int, a *kop.Ability, d *payDesc) bool {
	pp := &s.perms[sl]
	if pp.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 || s.genLost(k, sl) {
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
	return d == nil || a.ManaRestrict == 0 || s.restrSrcAdmits(k, p, sl, a, d)
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
			return true // a ColorIdentity source the payment-bound fold prices (CR 903.4)
		}
		c := s.cardAt(sl)
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
		// commander-only mana refused, Hydraulic Helper's admitted, CR 106.6)
		cd := &k.t.Cards[c]
		for j := range cd.Abilities {
			if ab := &cd.Abilities[j]; j < 16 && ab.Kind == kop.AbMana && ab.AFlags&kop.AfLatent == 0 &&
				ab.ManaRestrict != 0 && s.restrSrcAdmits(k, p, sl, ab, d) && s.potentialAdmits(k, p, sl, j, ab, nil) {
				return true
			}
		}
	}
	return false
}
