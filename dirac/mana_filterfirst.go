package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A filter as a card's first and only mana ability (CR
// 605.3a, 602.5b): the Devotees' "{1}: Add {W}, {B}, or {G}. Activate only
// once each turn", Three Tree Mascot's "{1}: Add one mana of any color.
// Activate only once each turn", Prophetic Prism's "{1}, {T}: Add one mana
// of any color" (kop.FilterManaAb). gorge's planner census prices none of
// them (pay.PlanLastResortCostOK rejects a mana part), so each is a paid
// census gap exactly like Hidden Grotto's second-ability filter (mana_filter.go),
// relaxed to one free unit of each colour it makes (potential_plan.go
// paymentPlanRelaxedAlternatives: its once-only cost is the {T} or the
// activation limit). It differs from Grotto's in three ways:
//
//   - it has no free ability: activating it poses only the colour ask
//     (rules activateManaFor resolves a one-ability source directly; the fee
//     is paid from the pool, no ask), Any's W U B R G or a Combo's colours
//     in printed order (effects.ComboColours; kop.Ability.ManaColOrd records
//     an out-of-WUBRG order, read by manaColourOrder), so filtStep's ans
//     stays 1+c;
//   - a once-each-turn one has no {T}: CR 302.6 does not bind it, a tapped
//     or summoning-sick Devotee still activates, and its use is
//     perm.actUsed (gorge's ManaActivate activation-limit marker), not the
//     tapped bit. gorge's search dedup (scriptStateKey) does not key on that
//     use, and filtKey does not either;
//   - gorge's census lists it only when its fee is payable from the
//     potential pool (paymentPlanCensusOf: manaAbilityPayablePool against
//     PotentialMana's fixpoint, where a source never funds its own fee):
//     Grotto's own {C} always pays its fee, a Devotee's needs other mana.
//
// It is never a funding step: it is always a census gap (a filter whose fee
// the potential pool cannot pay is not activatable in the search either),
// and the search funds with non-gap activate options only. In the payment
// window it is no unit (rules windowManaUnitsWith skips a non-free cost), as
// Grotto's filter is not: canTapForManaPC is false for it (pcard.manaTap
// needs a mana-free cost), so the plain walk never taps it, though the live
// window lists it once the pool pays its fee (windowFiltFirst).

// DebugFiltFirstNoLimit is a mutation (TestManaabmanacostMutation): a
// once-each-turn filter ignores its activation limit.
var DebugFiltFirstNoLimit bool

// prepFiltFirst records ability j of card c, its first mana ability, as a
// filter (genPrep.filt, gfFilt).
func (g *genPrep) prepFiltFirst(c CardID, j int, a *kop.Ability) {
	if j >= 8 {
		panic("kernel: a once-each-turn filter mana ability past ability 7 (perm.actUsed)")
	}
	g.smallFor(c).filt = int8(j)
	g.setFlag(c, gfFilt)
	g.anyFilt, g.anyFiltFirst = true, true
	if a.AFlags&kop.AfManaDynamic != 0 {
		g.anyDynFilt = true // the amount may resolve to zero: scriptFilt drops it then
	}
	if _, pip := pipFilt(a); pip {
		g.anyFiltPip = true
	}
	if kop.FeeOnlyManaAb(a) {
		g.anyFiltRepeat = true
	}
}

// filtAb is the filter ability of the card at sl.
func (s *State) filtAb(k *prep, sl Slot) *kop.Ability {
	c := s.cardAt(sl)
	return &k.t.Cards[c].Abilities[k.gen.filt(c)]
}

// filtNoTap: the filter at sl costs no {T} (a once-each-turn one).
func (s *State) filtNoTap(k *prep, sl Slot) bool {
	return k.gen.anyFiltFirst && s.filtAb(k, sl).Cost.Flags&kop.CostTap == 0
}

// filtFirstOnly: the filter at sl is its card's first mana ability.
func (s *State) filtFirstOnly(k *prep, sl Slot) bool {
	c := s.cardAt(sl)
	return k.gen.anyFiltFirst && firstManaAb(&k.t.Cards[c]) == int(k.gen.filt(c))
}

// filtUsable: the filter at sl can be activated now, its fee aside: a {T}
// one untapped and (a creature) unsick, a once-each-turn one unused this
// turn (gorge's ActivationLimit$ gate), and not locked by a CantBeActivated
// static (cast_cantactivate.go), as a plain mana source is not.
func (s *State) filtUsable(k *prep, sl Slot) bool {
	if s.genLost(k, sl) || k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return false
	}
	if a := s.filtAb(k, sl); a.Cost.Flags&kop.CostOncePerGame != 0 && s.onceBlocked(k, sl, a) {
		return false
	}
	if s.filtRepeat(k, sl) {
		return true // no {T}, no limit: always (mana_filterrepeat.go)
	}
	pm := &s.perms[sl]
	if s.filtNoTap(k, sl) {
		return DebugFiltFirstNoLimit || pm.actUsed&(1<<uint(k.gen.filt(s.cardAt(sl)))) == 0
	}
	return pm.flags&pfTapped == 0 && !(s.activationSick(k, sl))
}

// filtFirstMask is scriptFilt on a table with a first-only filter: p's
// usable filters, a first-only one only while other potential mana covers
// its fee (filtFirstAdmit).
//
//go:noinline
func (s *State) filtFirstMask(k *prep, p uint8) slotMask {
	var m, first slotMask
	var pot avail
	havePot := false
	for b := s.ctrlM[SeatIx(p)]; b.any(); b = b.rest() {
		sl := b.first()
		if s.pickedForMana(s.perms[sl].obj) {
			continue // a Convoke/Waterbend pick is tapped for that cast, not for mana
		}
		if !k.gen.has(s.cardAt(sl), gfFilt) || !s.filtUsable(k, sl) {
			continue
		}
		if s.filtRepeat(k, sl) {
			// gorge prices membership against the whole potential pool, which
			// an admitted any-colour fee-only source makes open
			if !s.filtRepeatPotential(k, p, sl, &pot, &havePot) {
				continue
			}
		} else if s.filtFirstOnly(k, sl) {
			first = first.with(sl)
			continue
		}
		m = m.with(sl)
	}
	if first.any() {
		m = m.or(s.filtFirstAdmit(k, p, first))
	}
	return m
}

// filtFirstAdmit is gorge's PotentialMana fixpoint over the first-only
// filters in pend: one is admitted once the other potential mana covers its
// fee, and an admitted one's whole production joins that pool for the rest
// (an upper bound: no fee is taken out, and a source never funds its own
// fee, CR 605.3a). Viridescent Bog's {B}{G}, its {1} paid by a Mountain,
// covers Magus of the Coffers' {2}, so both are census gaps and the search
// can chain them (CR 605.3b).
func (s *State) filtFirstAdmit(k *prep, p uint8, pend slotMask) slotMask {
	var in slotMask
	pool := s.filtOtherMana(k, p)
	for grew := true; grew; {
		grew = false
		for b := pend; b.any(); b = b.rest() {
			sl := b.first()
			if pool+s.filtFeeRestrSrc(k, p, sl) < s.filtFee(k, sl) {
				continue
			}
			in, pend = in.with(sl), pend.without(sl)
			pool += s.filtUnits(k, sl)
			grew = true
		}
	}
	return in
}

// filtOtherMana bounds the potential mana a first-only filter's fee may
// draw on before any other first-only filter is admitted (filtFirstAdmit
// adds theirs): the pool and every source the plain walk taps, plus the
// counter-cost, tap-another, deferred and rider gaps.
func (s *State) filtOtherMana(k *prep, p uint8) int {
	pl := &s.pl[SeatIx(p)]
	n := poolTotal(pl) + s.rbTotal(p, s.rbAll(p))
	for b := s.ctrlM[SeatIx(p)].and(s.occ); b.any(); b = b.rest() {
		sl := b.first()
		if s.perms[sl].flags&pfTapped == 0 && s.canTapForMana(k, sl) {
			// the whole production the census's plain arm adds (mana.go's
			// availForMode): the tap's own units and the all-at-once rest
			// (mana_eachcolour.go), as gorge's PotentialMana counts it (CR
			// 605.3b, 605.1a). Counting only manaN read an "Add {W}{U}" land as
			// one mana, so its {U} never funded a first-only filter's fee
			// (Cabal Coffers' {2} beside every Swamp tapped: Korlash, Heir to
			// Blackblade's {1}{B} activation offered by gorge, never by the
			// kernel).
			pc := s.manaPCGen(k, sl)
			n += int(s.srcYieldAt(k, sl, pc))
			if k.gen != nil && k.gen.anyTapExtra {
				xv := s.srcExtra(k, sl, pc)
				for c := range xv {
					n += int(xv[c])
				}
			}
		} else if pc := s.manaPCGen(k, sl); s.canSacOnlyForMana(k, sl, pc) {
			// a sacrifice-only source (an Eldrazi Spawn) is a last-resort
			// unit of gorge's census, tapped or not (mana_sacsource.go)
			n += int(max(pc.manaN, 1))
		} else if s.grantSacOK(k, sl) || s.grantTapOK(k, sl) {
			// a granted mana ability is potential mana gorge's planner counts
			// (Rain of Filth's "Sacrifice this land: Add {B}{B}" on a tapped
			// land): the plain walk above counts only a printed tap, so its
			// production funds a first-only filter's fee here too (CR 605.1a,
			// 601.2g). The script search still prices the actual payment.
			var ga avail
			s.availGrant(k, sl, s.manaPCGen(k, sl), &ga)
			n += ga.total
		}
	}
	if k.ctrMana {
		// a dead counter-cost gap (Component Pouch) is admitted to gorge's
		// PotentialMana with a production that resolves to no mana, so it
		// funds no fee (mana_countercost.go)
		n += s.scriptCtr(k, p).count() - s.deadCtrMask(k, p).count()
	}
	if k.tapMana {
		// a tap-another-permanent source (Relic of Legends' "Tap an untapped
		// legendary creature: Add one mana of any color") is potential mana
		// gorge's planner prices, so it funds a first-only filter's fee (CR
		// 605.1a, 601.2g): Overflowing Basin's {1} paid by the relic's mana,
		// then the Basin's {G}{U} casts Aberrant. The upper bound counts every
		// activatable tap-another source, its candidate taps aside (the script
		// search prices the actual payment).
		for b := s.scriptTapX(k, p); b.any(); b = b.rest() {
			var tx avail
			addTapXUnits(&tx, k.tapXAbOf(s.cardAt(b.first())))
			n += tx.total
		}
	}
	if k.gen.anyComboAny && s.caGaps(k, p).any() {
		// a Combo Any source (Axebane Guardian) or a reflected Amount$ one
		// (Overgrown Battlement): gorge's PotentialMana prices an any-combination
		// or indeterminate-amount production unbounded (addPotentialMana,
		// producedOpen), so the source alone covers a first-only filter's fee and
		// the filter is a census gap beside it (CR 605.3a, 601.2g). DebugComboAnyNoPotential
		// (mutation test only) leaves it uncounted.
		if !DebugComboAnyNoPotential {
			n += potentialUnboundedUnits
		}
	}
	if k.gen.anyEachCol && s.eachGapOpen(k, p) {
		n += potentialUnboundedUnits
	}
	if k.gen.anyDeferred {
		// a deferred source's {R} (mana_deferred.go): gorge's PotentialMana
		// counts it, so Pyromancer's Goggles alone funds Three Tree Mascot's {1}
		n += s.scriptDeferred(k, p).count()
	}
	if k.gen.anyManaRider {
		// a Taps-deferred rider gap's production (a depletion land's {G}{G},
		// Peat Bog's {B}{B}): gorge's PotentialMana counts a bounded rider
		// source (availPotentialFor's rider arm), so such a land alone funds a
		// first-only filter's {1} fee (CR 605.3a). The count walks the same
		// scriptTapsDefer census the gap search dispatches on
		// (mana_filtertapsdefer.go), so a rider beside a plain first ability
		// counts its rider production too.
		n += s.filtRiderMana(k, p)
	}
	return n
}

// windowFiltFirstUsable: the live payment window lists the first-only filter at
// sl: it is usable and the floating mana pays its fee (gorge's window abilities,
// manaAbilityPayablePool). It is still no unit of the census above. A filter
// whose dynamic amount resolves to nothing is not listed: gorge's
// windowSourceYields drops a source that adds no mana (payment_window_yield.go).
func (s *State) windowFiltFirstUsable(k *prep, p uint8, sl Slot) bool {
	c := s.cardAt(sl)
	return k.gen.has(c, gfFilt) && s.filtFirstOnly(k, sl) && s.filtUsable(k, sl) && s.filtPaid(k, p, sl) &&
		!s.dynManaYieldsNothing(k, s.filtAb(k, sl), sl) // a dynamic amount at zero makes nothing, so the window never lists it (Cabal Coffers with no Swamp, CR 605.1a)
}

// windowFiltFirst activates that filter in the window, for the first colour it
// makes that useful still owes (gorge paymentUsefulColours), in the ability's
// ask order (a Combo's printed order, colourAskOrder). Nothing owed answers the
// lowest W U B R G colour, as payexec.WindowAnswer does; a window whose ask
// carries no cost (a trigger's own cost, noOwed, with no clamp) has nothing to
// owe, so it takes the same lowest.
func (s *State) windowFiltFirst(k *prep, p uint8, sl Slot, useful *kop.ManaCost, noOwedUnclamped bool) {
	ab := s.filtAb(k, sl)
	mask := uint8(ab.ManaColors)
	c := uint8(bits.TrailingZeros8(mask))
	if !noOwedUnclamped && useful != nil {
		// gorge's ask lists what the cost still owes first (owedColoursFirst),
		// in the ability's own order, from the pool the payment may spend
		// (windowUseful, CR 106.6): a pip its restricted mana covers is not
		// owed. Nothing owed falls back to the lowest W U B R G colour, which
		// is what payexec.WindowAnswer answers then. A hybrid pip leaves the
		// verdict undecided (windowUseful prices nothing): gorge keeps every
		// colour there, so the census alone gives the demand.
		want, _ := s.windowUseful(k, p, useful)
		if useful.Hyb[0] != 0 {
			want = s.windowDemand(k, p, useful)
		}
		if a := askOwedColour(ab, mask, want); a >= 0 {
			c = uint8(a)
		}
	}
	s.filtDo(k, p, filtStep{sl: sl, ans: uint8(1 + c), gap: true})
}

// windowFiltFirstFallback is windowFiltFirst for a window whose ask carries no
// owed pips (a PaymentFallback with no ManaPayment: a cast's altAddWindowTap,
// and every noOwed window that keeps a clamp: convoke, Spree, a lowered or free
// cast, waterbend, pain): it answers the lowest colour of the source, narrowed
// only to what the clamp keeps once it has decided (windowUseful, gorge
// paymentUsefulColours). Gorge asks the colour after the fee is paid, so the
// clamp reads the pool the fee leaves, spent as filtDo spends it (the
// restricted batches the activation admits first, then the pool): an unsettled
// generic leaves every colour listed, and Abzan Devotee's {W}{B}{G} answers {W}
// while a {B} pip is still owed (CR 601.2g).
func (s *State) windowFiltFirstFallback(k *prep, p uint8, sl Slot, useful *kop.ManaCost) {
	mask := uint8(s.filtAb(k, sl).ManaColors)
	var want uint8
	var decided bool
	if _, pip := s.filtPip(k, sl); pip {
		want, decided = s.windowUseful(k, p, useful)
	} else {
		c := s.fork()
		fee := s.filtFee(k, sl)
		if col, ok := s.filtRepeatPip(k, sl); ok {
			c.filtPayPip(k, p, sl, col) // the pip first, as filtDo
			fee--
		}
		fee = c.filtFeeSpend(k, p, sl, fee) // filtDo's fee
		want, decided = c.windowUseful(k, p, useful)
		s.unfork()
	}
	if decided && mask&want != 0 {
		mask &= want
	}
	s.filtDo(k, p, filtStep{sl: sl, ans: uint8(1 + bits.TrailingZeros8(mask)), gap: true})
}

// filtUse spends the filter at sl: its {T}, or its use this turn.
func (s *State) filtUse(k *prep, sl Slot) {
	if s.filtRepeat(k, sl) {
		s.touchSl(sl) // repeatable: nothing marks a use (mana_filterrepeat.go)
		return
	}
	if s.filtNoTap(k, sl) {
		s.perms[sl].actUsed |= 1 << uint(k.gen.filt(s.cardAt(sl)))
	} else {
		s.perms[sl].flags |= pfTapped
		s.manaTapped(k, sl)
	}
	s.touchSl(sl)
}

// windowFiltAffords: gorge's ward window lists a first-only filter once the
// floating mana pays its fee (windowFiltFirstUsable), so the sources that fund
// the fee plus the filter's output can pay a ward cost the census (availFor:
// no filter is a unit) prices short. The walk payWindow runs is simulated on
// a copy (CR 605.3a, 702.21: the ward cost is paid through the mana window).
// The second return is the first rule-gap stall a failed walk's fork latched
// (StallNone none): the probe declares nothing — the caller may still price
// the play through a sibling route, and the drop site declares the stall only
// where every route failed (payWardN; tapXScript's shape).
func (s *State) windowFiltAffords(k *prep, p uint8, cost *kop.ManaCost) (bool, uint8) {
	if k.gen == nil || !k.gen.anyFiltFirst || !s.scriptFilt(k, p).any() {
		return false, 0
	}
	t := s.fork()
	defer s.unfork()
	ok, _ := t.windowTapWalk(k, p, cost, cost, false, false, true, false, true, false, false)
	stall := uint8(0)
	if !ok {
		stall = t.g.stalled
	}
	return ok, stall
}

// potentialUnboundedUnits stands for gorge's unbounded potential production
// (potentialUnbounded) in a first-only filter's fee test: far above any fee.
const potentialUnboundedUnits = 1 << 20

// DebugComboAnyNoPotential (mutation test only): a Combo Any source adds
// nothing to the potential mana a first-only filter's fee draws on.
var DebugComboAnyNoPotential bool

// eachGapOpen: p has an activatable each-colour source (Faeburrow Elder,
// "Produced$ Special EachColorAmong_Valid ..."). gorge's PotentialMana folds
// the ability through addPotentialMana (potential.go; computePotentialManaAdd),
// where producedOpen("Special...") is true: the source prices unbounded in every
// colour, so it alone covers a first-only filter's fee (Orzhov Signet's {1},
// CR 605.3a, 601.2g) and the filter is a census gap beside it. DebugEachGapNoPotential
// (mutation test only) leaves it uncounted.
func (s *State) eachGapOpen(k *prep, p uint8) bool {
	return !DebugEachGapNoPotential && s.scriptGaps(k, p).any()
}

// DebugEachGapNoPotential (mutation test only): an each-colour source adds
// nothing to a first-only filter's fee cover (filtOtherMana).
var DebugEachGapNoPotential bool
