package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// potentialDynOpen reports whether the trigger-cost offer gate
// (interp_ask.go trigCostPotentialPays) opens because p controls an untapped
// census source whose mana ability's amount is dynamic (AfManaDynamic, the
// gfManaVar family: Lotus Blossom's "Add X mana of any one color, where X is
// the number of petal counters on Lotus Blossom") while a source on the board
// still yields mana now. gorge's potential walk (rules potential.go
// addPotentialMana) prices such an amount potentialUnbounded -- an
// indeterminate Amount$ is never priced at zero, whatever it reads now --
// while the census prices the amount the source holds (gen.go varPC: zero
// petals is zero mana and no planner unit, manaPCGen), so the gate's fold
// (availPotentialFor) could not reach the cost and the ask was decline-only
// while gorge offered pay beside decline (CR 118.3: the payer elects pay, the
// charge fails, the payer declines).
//
// The dynamic source must be one gorge's own gate admits, and the admission is
// the potential walk's, not the census's: dynSourceAdmits reads the walk's
// membership gates (potentialRestrictionAdmits, potential.go) and the
// activation's payability (manaAbilityPayablePool) without the census's
// zero-amount exclusions.
func (s *State) potentialDynOpen(k *prep, p uint8, d *payDesc) bool {
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
			// a commander-identity production prices unbounded only when the
			// identity has colours (gorge potentialIdentityEmpty, CR 903.4f);
			// the shape stays decline-only here
			if a.ManaIdentity {
				continue
			}
			if dynamicManaCost(a) && s.dynSourceAdmits(k, p, sl, j, a, d) {
				return true
			}
		}
	}
	return false
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
