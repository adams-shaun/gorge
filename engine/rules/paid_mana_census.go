package rules

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// paid_mana_census.go is the ONE paid-cost layer over the shared payment
// census (windowManaUnits). The census is ON by default; Config.
// DisablePaidManaCensus restores the pre-flip narrow census. It widens what
// the attack-tax budget and tap list, the unless-cost windows, AvailableMana
// and the pool-short ability offer (legal_walk_autopay.go) read: a mana
// source whose activation cost is more than a bare tap is counted when the
// engine can pay that cost with no choice and no sub-ask -- a
// self-sacrifice (Sac<1/CARDNAME>: Treasure, Lotus Petal, Black Lotus) or a
// fixed PayLife<N> that leaves the payer alive.
//
// Every reader asks paidCostShape, and the cast window's own paid layer
// (castWindowProbeUnits) asks it too, so the windows cannot disagree about
// which sources are payable. Still excluded (fail closed): a Sac of another
// permanent, discard, exile, counter removal, tapping other permanents, a
// generic/coloured mana fee (a cast window prices that one itself), an
// announced or dynamic amount, RestrictValid$ and InstantSpeed$ abilities,
// and a production needing the source's as-enters colour.

// paidCostShape reports whether cost is a paid activation cost the census
// can pay deterministically right now by source id of payer p, and the life
// it spends. life is the payer's life still unspent: a PayLife<N> that would
// take it to 0 or below is not payable (CR 119.4 permits it, but the payer
// would lose; no census counts a source that kills its payer). A
// self-sacrifice requires the source to be the only match of its CARDNAME
// spec and its sacrifice to be unblocked (pay.CastWindowSelfSacCost).
func (e *Engine) paidCostShape(p state.PlayerID, id state.ObjID, cost Cost, life int32) (int32, bool) {
	switch {
	case castWindowPayLifeCost(cost):
		if life <= cost.Life {
			return 0, false
		}
		return cost.Life, true
	case pay.CastWindowSelfSacCost(asPayer(e), p, id, cost):
		return 0, true
	}
	return 0, false
}

// paidManaAbility classifies one of a source's window mana abilities for the
// paid-cost census: ok when ma is a non-free ability with a literal positive
// amount, no RestrictValid$, an available tap part and a paidCostShape cost;
// life is what it spends. lifeLeft is the payer's life not yet committed to
// other sources of this census.
func (e *Engine) paidManaAbility(p state.PlayerID, id state.ObjID, o *state.Object, ma *cards.SA, lifeLeft int32) (life int32, ok bool) {
	mf := e.manaStaticFor(p, id, ma)
	if mf.RestrictValid || mf.FreeCost || mf.Amount <= 0 {
		return 0, false
	}
	if (mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped) {
		return 0, false
	}
	if producedNeedsChosen(mf.Produced) {
		return 0, false
	}
	total := int32(0)
	for _, n := range mf.Counts {
		total += n
	}
	if total <= 0 {
		return 0, false
	}
	cost := e.manaAbilityCC(p, id, ma).Cost
	if pay.ActivationTapCostUnavailable(o, &cost) {
		return 0, false
	}
	return e.paidCostShape(p, id, cost, lifeLeft)
}

// windowManaUnitsPaid is the census the attack-tax and unless-cost readers
// share: windowManaUnits, plus -- unless Config.DisablePaidManaCensus
// -- the paid-cost layer. With the census disabled it IS windowManaUnits.
func (e *Engine) windowManaUnitsPaid(p state.PlayerID) []windowManaUnit {
	units := e.windowManaUnits(p)
	if !e.paidManaCensus {
		return units
	}
	return e.appendPaidManaUnits(p, units)
}

// appendPaidManaUnits adds the paid-cost alternatives to units, walking p's
// battlefield in zone order (deterministic). A source taps once, so its
// life is the MAX over its alternatives; the life committed by earlier
// sources is withheld from later ones so every counted source can be paid
// together and the payer still survives.
func (e *Engine) appendPaidManaUnits(p state.PlayerID, units []windowManaUnit) []windowManaUnit {
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	committed := int32(0)
	life := e.G.Players[p].Life
	ids := pay.AppendBattlefieldManaSourceIDs(e.G, e.idsBorrow(), p)
	defer e.idsRelease(ids)
	for _, id := range ids {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		maxLife := int32(0)
		for _, ma := range e.availableManaAbilitiesForWindow(p, id, false) {
			lc, ok := e.paidManaAbility(p, id, o, ma, life-committed)
			if !ok {
				continue
			}
			for _, alt := range e.paidManaAlts(id, ma, lc) {
				units = pay.AppendUnitAlt(units, id, alt)
				if lc > maxLife {
					maxLife = lc
				}
			}
		}
		committed += maxLife
	}
	return units
}

// paidManaAlts expands one classified ability into its census alternatives:
// a plain production is one alt; a choice-shaped production is one alt per
// producible colour whose ability is pinned to that colour (the attack
// window's own pin, so the activation poses no colour sub-ask). An ability
// carrying an activation limit is not pinned (the limit's pile identity
// would be lost on the copy) and is left out.
func (e *Engine) paidManaAlts(id state.ObjID, ma *cards.SA, life int32) []windowManaAlt {
	mf := e.manaStaticOf(ma)
	alt := windowManaAlt{Ma: ma, Counts: mf.Counts, Amt: mf.Amount, Life: life, LifeKeep: life > 0}
	if !mf.Any {
		return []windowManaAlt{alt}
	}
	if mp := effects.ManaOf(ma); mp.Produced == "" {
		// The executor's deterministic one-colourless default.
		return []windowManaAlt{alt}
	}
	return pinnedManaAlts(alt, mf.Counts)
}

// fixedMultiColourAlt is the one alt of a fixed production over several
// colours ("{1}, {T}: Add {W}{B}", Produced$ W B): it makes BOTH colours, so
// splitting it per colour (pinnedManaAlts, for a choice) would price it at one
// mana and lose the filter land's second. ok is false for a single-colour
// production and for an ability carrying an activation limit (left out, as
// pinnedManaAlts does).
func fixedMultiColourAlt(alt windowManaAlt, counts [6]int32) (windowManaAlt, bool) {
	colours := 0
	for _, n := range counts {
		if n != 0 {
			colours++
		}
	}
	ma := alt.Ma
	if colours < 2 || ma.ParamStr(cards.PKGameActivationLimit) != "" || pay.ChainGatesOnActivationCount(ma) {
		return windowManaAlt{}, false
	}
	if _, limited := ma.Param(cards.PKActivationLimit); limited {
		return windowManaAlt{}, false
	}
	alt.Counts = counts
	return alt, true
}

// pinnedManaAlts splits a choice-shaped production over the colours in counts
// into one alt per colour, each pinned so that its activation poses no colour
// ask. An ability carrying an activation limit is left out.
func pinnedManaAlts(alt windowManaAlt, counts [6]int32) []windowManaAlt {
	ma := alt.Ma
	if _, limited := ma.Param(cards.PKActivationLimit); limited || ma.ParamStr(cards.PKGameActivationLimit) != "" ||
		pay.ChainGatesOnActivationCount(ma) {
		return nil
	}
	var out []windowManaAlt
	for c, n := range counts {
		if n == 0 {
			continue
		}
		var single [6]int32
		single[c] = 1
		a := alt
		a.Counts = single
		a.Ma = pay.WithProduced(ma, ma, string(cards.ManaSymbol(c)))
		out = append(out, a)
	}
	return out
}

// fixedManaAlts prices a FIXED production (ProducedCounts' any=false: no
// colour is chosen on activation) as ONE alternative over its full counts.
// The activation produces every slot of counts at once, so per-colour
// pinning would price a two-colour tap for one unit (Sunscorched Divide's
// "{1}, {T}: Add {R}{W}" under an unless cost, CR 605.3a) and hide half the
// production behind the other alternative. It keeps pinnedManaAlts'
// activation-limit exclusion, the one eligibility rule its callers shared.
func fixedManaAlts(alt windowManaAlt, counts [6]int32) []windowManaAlt {
	ma := alt.Ma
	if _, limited := ma.Param(cards.PKActivationLimit); limited || ma.ParamStr(cards.PKGameActivationLimit) != "" ||
		pay.ChainGatesOnActivationCount(ma) {
		return nil
	}
	alt.Counts = counts
	return []windowManaAlt{alt}
}

// manaSubCounterCarried reports whether o carries every fixed counter the
// cost's source-anchored SubCounter parts name, so the activation's settle
// (PayManaSourceParts) removes a counter that exists.
func manaSubCounterCarried(o *state.Object, cost Cost) bool {
	for _, part := range cost.SubCounter {
		if o.Counter(part.Spec) < part.N {
			return false
		}
	}
	return true
}

// unlessWindowUnits is the census the unless-cost windows read:
// windowManaUnitsPaid plus the free abilities whose colour is chosen on
// activation (CR 605.3a), which that census withholds. They arrive pinned, one
// alt per colour: the shape the reach search prices, and a tap in the
// component window that poses no colour ask. So do the bare-tap abilities whose
// Amount$ only the evaluator can price (Doc Samson's X mana of one colour, X its
// power), as the payment planner's evaluated-amount layer counts them. Each
// tap's static TapsForMana extra joins its alternative (windowTapExtras), the
// same fold the cast window applies: the reach search must price the source at
// the mana plus the extra its tap brings (CR 106.12a, 605.4a).
func (e *Engine) unlessWindowUnits(p state.PlayerID) []windowManaUnit {
	units := e.windowManaUnitsPaid(p)
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	ids := pay.AppendBattlefieldManaSourceIDs(e.G, e.idsBorrow(), p)
	defer e.idsRelease(ids)
	var probe *Engine
	for _, id := range ids {
		o := e.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, ma := range e.availableManaAbilitiesForWindow(p, id, false) {
			mf := e.manaStaticFor(p, id, ma)
			// A restricted batch joins the evaluated-amount arm only when its
			// restriction admits the PurposeOther payment (CR 106.6: Vhal,
			// Candlekeep Researcher's toughness-sized {C} can't cast a spell
			// from hand but pays an unless tax).
			if mf.Amount <= 0 && mf.TapOnly && !o.Tapped && (!mf.RestrictValid ||
				pay.UnlessRestrictionAdmitsOther(asPayer(e), p, effects.ManaOf(ma).RestrictValid, id)) {
				if probe == nil {
					probe = e.paymentPlanTappedProbe(p)
				}
				if amt, ok := e.paymentPlanStableAmount(probe, p, id, o, ma); ok {
					for _, a := range pinnedManaAlts(windowManaAlt{Ma: ma, Amt: amt}, effects.ManaOf(ma).Counts) {
						units = pay.AppendUnitAlt(units, id, a)
					}
				}
				continue
			}
			if mf.RestrictValid {
				// CR 106.6: a restriction on the ability's batch bars only the
				// payment classes it names. CantCastNonArtifactSpells bars
				// casting a nonartifact spell -- not the unless tax, ward or
				// attack cost this census prices -- so an admitted source
				// joins the census its unrestricted siblings join; a
				// restriction naming no PurposeOther payment keeps its units
				// out (fail closed). The read is pay.UnlessRestrictionAdmitsOther,
				// the same predicate AvailableFor applies to a floating
				// batch, and the tap ask offers the alt for the same
				// membership -- the offer gate cannot promise more than the
				// window can tap.
				if pay.UnlessRestrictionAdmitsOther(asPayer(e), p, effects.ManaOf(ma).RestrictValid, id) {
					if a, ok := unlessRestrictedAlt(ma, o.Tapped, mf); ok {
						units = pay.AppendUnitAlt(units, id, a)
					}
				}
				continue
			}
			// A fixed source-anchored SubCounter fee is the one further paid
			// shape this census can price deterministically (Remote Farm's
			// "{T}, Remove a depletion counter from CARDNAME: Add {W}{W}",
			// CR 608.2g): ManaSubCounterOnlyCost keeps the cost ask-free and
			// mana-free, so the reach search prices the literal production
			// alone and the activation's settle (PayManaSourceParts) removes
			// counters the source carries -- checked here, exactly what that
			// settle assumes. A choice-shaped production or a fixed part the
			// source cannot pay stays out (fail closed, the census
			// convention).
			if !mf.Any && !mf.FreeCost && mf.Amount > 0 &&
				!((mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped)) {
				cost := e.manaAbilityCC(p, id, ma).Cost
				if pay.ManaSubCounterOnlyCost(cost) && manaSubCounterCarried(o, cost) {
					for _, a := range fixedManaAlts(windowManaAlt{Ma: ma, Amt: mf.Amount}, mf.Counts) {
						units = pay.AppendUnitAlt(units, id, a)
					}
				}
			}
			if mf.FreeCost && mf.Amount == 1 && !((mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped)) &&
				pay.IsColourIdentityProduced(effects.ManaOf(ma).Produced) {
				// "Add one mana of any color in your commander's color
				// identity" (Command Tower, CR 903.4, 605.3a) is a finite
				// colour pick: one pinned alt per identity colour, so the
				// offer gate prices it exactly as the activation it later
				// taps. No identity colour prices nothing (fail closed).
				var counts [6]int32
				for _, c := range pay.CommanderIdentityColours(asPayer(e), p) {
					if i := strings.Index("WUBRG", c); i >= 0 {
						counts[i] = 1
					}
				}
				for _, a := range pinnedManaAlts(windowManaAlt{Ma: ma, Amt: mf.Amount}, counts) {
					units = pay.AppendUnitAlt(units, id, a)
				}
				continue
			}
			if !mf.Any || !mf.FreeCost || mf.Amount <= 0 ||
				(mf.Tap && o.Tapped) || (mf.Untap && !o.Tapped) {
				continue
			}
			mp := effects.ManaOf(ma)
			counts := mf.Counts
			if mp.NeedsChosen {
				chosen := pay.ChosenProducedColour(e.G, id)
				if chosen == "" {
					continue
				}
				counts, _ = cards.ProducedCounts(pay.SubstituteChosenProduced(mp.Produced, chosen))
			}
			// A lone colourless slot is the executor's default, which the
			// shared census already counts.
			if counts == ([6]int32{5: 1}) {
				continue
			}
			for _, a := range pinnedManaAlts(windowManaAlt{Ma: ma, Amt: mf.Amount}, counts) {
				units = pay.AppendUnitAlt(units, id, a)
			}
		}
		// A literal mana fee (Apprentice Wizard's "{U}, {T}: Add {C}{C}{C}")
		// is the one further non-free shape this census prices, and it walks
		// the GATE-BYPASSING member list: a fee the current pool cannot yet
		// cover is exactly the source this census must still list, because
		// the reach search pays the fee from the floating mana an earlier
		// same-window activation produced (castWindowReachable's fee
		// transition), and the live window offers the source once that fee
		// is payable (manaAbilityPayablePool in the live member list), so
		// the offer never promises an activation the window will not pose.
		// The fee rides the alt (CostGeneric/CostColored, never netted into
		// the production). A restricted ability and an unmodelled Condition$
		// stay out (fail closed, the census convention). A colour chosen on
		// activation joins as PINNED one-alt-per-colour alternatives, the
		// same shape the free any-colour arm above prices, so the activation
		// poses no colour sub-ask; its self-sacrifice joins through
		// CastWindowSelfSacFeeCost, the deterministic shape whose only match
		// is the source itself (Chromatic Star's "{1}, {T}, Sacrifice
		// Chromatic Star: Add one mana of any color", CR 605.3a/b: the payer
		// may activate mana abilities to meet the payment).
		for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
			if pay.InstantSpeedOnly(ma) || manaAbilityAddsNothing(ma) {
				continue
			}
			fmf := e.manaStaticFor(p, id, ma)
			if fmf.RestrictValid || fmf.FreeCost || fmf.Amount <= 0 || effects.ManaOf(ma).NeedsChosen {
				continue
			}
			fee := e.manaAbilityCC(p, id, ma).Cost
			if pay.ActivationTapCostUnavailable(o, &fee) {
				continue
			}
			// The fee's self-sacrifice shape is priced here, at the one call
			// that owns the engine: the alts builder below stays free of it.
			selfSac := pay.CastWindowSelfSacFeeCost(asPayer(e), p, id, fee)
			for _, a := range unlessFeeAlts(ma, fmf, fee, selfSac) {
				units = pay.AppendUnitAlt(units, id, a)
			}
		}
	}
	if len(pay.PaymentPlanInterferenceCarriers(asPayer(e))) > 0 {
		units = e.windowTapExtras(units)
	}
	// CR 106.1: a reflected tap (Fellwar Stone) adds a coloured mana, not
	// the census's colourless stand-in; the offer gate must price it so. The
	// pass reads only the payer, so a payer-only pendingCast carries it.
	return e.castWindowReflectedUnits(&pendingCast{player: p}, units)
}

// unlessFeeAlts prices one gate-bypassing activation carrying a literal mana
// fee: the fee rides the alt (CostGeneric/CostColored, never netted into the
// production). A fixed production over several colours ("Add {W}{B}") is
// every colour at once, not a pick among them: one alt carries the whole
// vector (CR 106.1); pinning it per colour would price a Desolate Mire at one
// mana. A colour chosen on activation is the reverse: one PINNED alt per
// colour, so the activation poses no colour sub-ask. selfSac is the caller's
// verdict that the fee's own cost is the self-sacrifice shape whose only
// match is the source itself (Chromatic Star); without it or the bare-mana
// shape the cast window prices, an unpriced fee yields no alts (fail closed,
// the census convention).
func unlessFeeAlts(ma *cards.SA, fmf pay.ManaStatic, fee Cost, selfSac bool) []windowManaAlt {
	if !castWindowGenericCostShape(fee) && !selfSac {
		return nil
	}
	feeAlt := windowManaAlt{Ma: ma, Amt: fmf.Amount, CostGeneric: fee.Generic, CostColored: fee.Colored}
	if fmf.Any {
		return pinnedManaAlts(feeAlt, fmf.Counts)
	}
	if a, ok := fixedMultiColourAlt(feeAlt, fmf.Counts); ok {
		return []windowManaAlt{a}
	}
	return pinnedManaAlts(feeAlt, fmf.Counts)
}

// unlessRestrictedAlt prices one PurposeOther-admitted restricted mana
// ability as the shared census prices its unrestricted siblings
// (windowManaUnitsWith): a deterministic production only -- a fixed colour
// set, or the executor's one-colourless default -- on a free cost the source
// can pay right now. A colour-choosing production stays out: the payer's
// colour choice is a sub-ask this census does not price, so counting it
// would promise mana the tap cannot commit without one.
func unlessRestrictedAlt(ma *cards.SA, tapped bool, mf pay.ManaStatic) (windowManaAlt, bool) {
	if !mf.FreeCost || (mf.Tap && tapped) || (mf.Untap && !tapped) {
		return windowManaAlt{}, false
	}
	if mf.Amount <= 0 {
		return windowManaAlt{}, false
	}
	counts, any := mf.Counts, mf.Any
	total := int32(0)
	for _, n := range counts {
		total += n
	}
	if any {
		// Only the executor's deterministic one-colourless default counts.
		if total != 1 || counts[5] != 1 {
			return windowManaAlt{}, false
		}
	} else if total <= 0 {
		return windowManaAlt{}, false
	}
	return windowManaAlt{Ma: ma, Counts: counts, Amt: mf.Amount, Any: any}, true
}

// availablePaidAbilities is AvailableMana's paid-cost extension: the
// abilities of source id (already in free's reading order) the paid-cost
// census would also count, with the life they commit.
func (e *Engine) availablePaidAbilities(p state.PlayerID, id state.ObjID, o *state.Object, lifeLeft int32) ([]*cards.SA, int32) {
	var out []*cards.SA
	maxLife := int32(0)
	for _, ma := range e.availableManaAbilities(p, id) {
		lc, ok := e.paidManaAbility(p, id, o, ma, lifeLeft)
		if !ok {
			continue
		}
		out = append(out, ma)
		if lc > maxLife {
			maxLife = lc
		}
	}
	return out, maxLife
}
