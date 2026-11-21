package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The ward / unless mana window's Bloom Tender and Giada path and the other cold
// window code live here. The linker lays a package's functions out in file order,
// so code added to a file in the middle shifts every hot function after it
// by its size, and a 32-byte shift of the planner and step functions'
// alignment cost BenchmarkTransitionFDN 2-3 % (interleaved, core 24,
// GOMAXPROCS=1) although none of this code runs on the benchmark. Keep
// rarely run code here, and keep the hot functions it replaced at their
// old sizes (execPay calls windowPays, prepareGen restrTapMana).

// windowTap activates p's sources in the window's order until the pool
// (without the restricted mana, returned in held and kept out of the pool)
// pays cost; ok is false when the sources run out first. clamp is a
// cast's window: a source that cannot add a colour the payment still
// absorbs is not offered (windowUseful).
func (s *State) windowTap(k *prep, p uint8, cost *kop.ManaCost, clamp bool) (ok bool, held [kop.NumManaKinds]uint8) {
	return s.windowTapRider(k, p, cost, clamp, false)
}

// windowTapNoOwed is windowTap for a payment window whose ask carries no
// owed cost: gorge's trigger-cost manual window (rules resPayFund ->
// paymentManaAskClass) poses a bare activate/done list with no ManaPayment,
// so its colour ask has nothing owed and payWindowAnswer takes the first
// colour (W). That is the route a trigger's pay election falls back to when
// the planner has no witness (act_discover.go payReplanned); a ward or unless
// window keeps windowTap's owed-colour answer.
func (s *State) windowTapNoOwed(k *prep, p uint8, cost *kop.ManaCost) (ok bool, held [kop.NumManaKinds]uint8) {
	return s.windowTapUseful(k, p, cost, cost, false, false, true, true)
}

// altAddWindowTap is the CR 601.2g window of a cast whose chosen either-or
// part cannot be paid. Gorge reverses that cast (CR 733.1) only after the
// window: it taps the useful sources in order and keeps the mana (CR 733.1's
// mana-ability reversal is a "may", and gorge does not), so the taps stay.
// A cast with a required target never reaches the window: the target menu
// withholds every candidate of an unpayable cast and the cast aborts at the
// target ask ("no legal target", cast_targets.go), tapping nothing.
func (s *State) altAddWindowTap(k *prep) {
	e := &s.bld().e
	if e.kind != seSpell || s.requiredTarget(k) {
		return
	}
	c := s.buildCost(k)
	// The behold-or-mana fallback window is the only either-or shape this
	// walk models: the other shapes' parts are the parent's offer gate's
	// business, and the window's widened behaviour below is gated on BeholdN.
	behold := k.t.Cards[e.card].BeholdN != 0
	// Gorge's fallback window (a chosen part that changed the plan's cost)
	// carries a PaymentFallback but no ManaPayment, so the oracle's windowOwed
	// reads only the flexible pip faces the payer announced: with none, the
	// colour ask takes the source's lowest colour, not the pip the printed
	// cost still owes. A window whose plan still matches (the chosen part is
	// the priced one) carries the ManaPayment and answers the owed pip, so
	// the no-owed answer applies only to the fallback: the mana part chosen
	// while the free part is still payable (planCastAltAdd priced the free
	// part, and the choice changed the cost). CR 601.2b/h, 601.2g.
	noOwed := behold && s.bld().altAdd == 1 && s.bld().pipN == 0 && s.altAddFreePart(k, e.ctrl, e.card, e.obj)
	// The real window runs at the mana stage (CR 601.2g), after the push: a
	// tap-another-permanent source (Springleaf Drum) is listed there and its
	// creature tapped as part of the activation. This synchronous walk runs
	// at the choice stage instead, so it borrows the mana stage's
	// castTapX listing and takes the first candidate the oracle's
	// WindowAnswer would answer (the build cannot pause for a pick here).
	b := s.bld()
	saved := b.stage
	if behold {
		b.stage = bsAltAdd
	}
	s.windowTapUseful(k, e.ctrl, &c, &c, true, false, noOwed, false)
	b.stage = saved
}

// DebugAltAddNoPipsFirst reverses an unpayable either-or part at the choice,
// before the pips (TestTitaniaLockstepMutation).
var DebugAltAddNoPipsFirst bool

// altAddPipsFirst: a hybrid spell with no required target announces its pips
// (gorge manaAsk) after the either-or choice, so choosing a part it cannot pay
// reverses the cast only once they are answered, in the payment (CR 601.2b, 601.2g).
func (s *State) altAddPipsFirst(k *prep) bool {
	e := &s.bld().e
	return !DebugAltAddNoPipsFirst && e.kind == seSpell && k.gen != nil && k.t.Cards[e.card].HasAltAdd && !s.requiredTarget(k) && k.t.Cards[e.card].Cost.Hyb[0] != 0
}

// altAddOpenTargets: gorge cannot tell the unpayable part from a payable one yet.
// A restricted source this payment may spend leaves its window census incomplete,
// and with any source untapped the target menu keeps its candidates
// (affordableTargetCandidates): the cast reverses in the window, after the target
// is chosen. Only the mana can wait for the window: a part whose other costs
// can't be paid (Bitter Triumph's discard as the last card in hand) empties the
// menu, so the cast reverses at once.
func (s *State) altAddOpenTargets(k *prep) bool {
	e := &s.bld().e
	if e.kind != seSpell || k.gen == nil || !s.requiredTarget(k) {
		return false
	}
	if k.t.Cards[e.card].Cost.Hyb[0] != 0 {
		return true // the pip ask comes first, then the menu (altAddTargetAbort)
	}
	return s.altAddPickPayable(k, s.bld().altAdd) && s.windowUnpricedOpen(k, e)
}

// windowUnpricedOpen: the window census is incomplete and a source is left to
// tap, so gorge keeps the pool-short candidates of a target menu or an either-or
// part on offer (affordableTargetCandidates' census fallback, CR 601.2g).
func (s *State) windowUnpricedOpen(k *prep, e *stackEntry) bool {
	return s.windowUnpricedOpenIf(k, e, !s.windowCensusComplete(k, e))
}

// copyCensusOpen is windowUnpricedOpen for a paid copy cast's target menu: a
// blocked self-sacrifice any-colour mana source (a Treasure under Quark,
// sacAnyManaGap) is a census gap too (gorge castWindowSourceExcluded's
// CountsAny arm), so the menu keeps its candidates however short the pool is.
// Only this gap is read on the copy path; the copy abort's other census
// classes keep their existing affordability arm.
func (s *State) copyCensusOpen(k *prep, e *stackEntry) bool {
	return s.windowUnpricedOpenIf(k, e, k.sacMana && s.sacAnyManaGap(k, e.ctrl))
}

// windowUnpricedOpenIf is windowUnpricedOpen with the census gap decided by
// the caller: open reports the unpriced census, and a source must be left to
// tap (gorge's hasUntappedManaSource): a tap, an each-colour or restricted
// gap, a tap-another one, or an ability whose cost needs no tap -- a
// sacrifice-only source (CR 602.2b) or a life-only pain one, which a tapped
// permanent holds as well (gorge mana_available.go rejects only
// mf.Tap && o.Tapped). An idle storage battery counts too, empty or not:
// gorge prices its SubCounter<X> cost at X=0, so the untapped permanent is a
// source the census cannot price (mana_storecensus.go, castWindowCensusComplete).
func (s *State) windowUnpricedOpenIf(k *prep, e *stackEntry, open bool) bool {
	if !open {
		return false
	}
	tapX := s.windowTapXLive(k, e.ctrl)
	ctr := s.scriptCtrIdle(k, e.ctrl, true)
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		if sl := m.first(); s.canTapForMana(k, sl) || s.windowGap(k, e.ctrl, sl) || tapX.has(sl) || s.restrManaUntapped(k, sl) ||
			ctr.has(sl) || k.sacOnlyMana && s.canSacOnlyNowAt(k, sl) || k.painMana && s.canLifeOnlyMana(k, sl, SeatIx(e.ctrl)) {
			return true
		}
	}
	return s.handManaAny(k, e.ctrl) // a hand guide (mana_handguide.go)
}

// restrManaUntapped: sl is an untapped permanent with a restricted mana ability.
// gorge's untapped-source gate (untappedManaSource) reads every available mana
// ability, its restriction unread, so Interdimensional Web Watch's exile-cast
// mana keeps a pool-short candidate on the menu once the census is incomplete.
// A latent line is never the carrier's own ability -- Resonating Lute's granted
// {T} lives on the lands its Continuous static reaches (kop.AfLatent), so an
// untapped Lute beside only tapped lands is no restricted source at all.
func (s *State) restrManaUntapped(k *prep, sl Slot) bool {
	pm := &s.perms[sl]
	if pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) || DebugNoRestrManaOpen {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 &&
			(a.ManaRestrict != 0 || a.ManaRestrictAb != 0) {
			return true
		}
	}
	return false
}

// DebugNoRestrManaOpen leaves a restricted mana source out of the window's
// untapped-source gate (mutation testing only: the lockstep must catch it).
var DebugNoRestrManaOpen bool

// altAddTargetAbort: the pips of a hybrid spell are announced and its chosen part
// still can't be paid, so the target menu withholds every candidate
// (affordableTargetCandidates) and a required target reverses the cast.
func (s *State) altAddTargetAbort(k *prep) bool {
	b := s.bld()
	return b.altAdd == 1 && b.pipN != 0 && b.e.kind == seSpell && k.gen != nil && k.t.Cards[b.e.card].HasAltAdd &&
		!s.altAddPayable(k, 1) && (!s.altAddPickPayable(k, 1) || !s.windowUnpricedOpen(k, &b.e))
}

// loweredPipWindow pays a lowered hybrid cast's pips through the window before its
// cost objects are chosen (CR 601.2g-h): the witness priced the pips its own way,
// so an answer it didn't cover opens the window. A window that runs out of useful
// sources first does not reverse the cast there: gorge's window closes with nothing
// left to offer (manaWindowAsk), the cost objects are still asked (a discard), and
// the payment then finds the mana short and reverses the cast before paying any of
// them (CR 601.2h, 733.1), the taps standing. finishBuild reads pipShort.
func (s *State) loweredPipWindow(k *prep, e *stackEntry, cost *kop.ManaCost) {
	anyColor := s.bld().bfl&bflAnyColor != 0
	if cost.X != 0 || s.loweredPoolPays(k, e, *cost, anyColor) {
		return
	}
	ok, held := s.windowTapUseful(k, e.ctrl, cost, cost, e.flags&sfAnyMana == 0 && !anyColor, anyColor, false, false)
	for c := range held {
		s.pl[e.ctrl].pool[c] += held[c]
	}
	s.bld().pipShort = !ok
}

// requiredTarget: the entry being built has a target slot it must fill at
// announcement (bsTargets would pose it and abort without a legal one).
func (s *State) requiredTarget(k *prep) bool {
	e := &s.bld().e
	a := s.buildAbility(k)
	for ti := 0; ti < kop.MaxTargets; ti++ {
		t := tspecE(k, e, ti)
		if t == nil || t.Kind == kop.TgtNone || t.UpTo || t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if a != nil {
			if tm := a.TgtModes[ti]; tm != 0 && tm&a.ModeSet(e.mode) == 0 {
				continue
			}
		}
		return true
	}
	return false
}

// windowTapRider is windowTap with the may-play any-colour rider: the pool
// pays when its coloured mana covers the coloured pips in any colours
// ({C} stays strict), and every source's colour is usable, so the clamp is
// off and the source colour is the lowest it makes.
func (s *State) windowTapRider(k *prep, p uint8, cost *kop.ManaCost, clamp, anyColor bool) (ok bool, held [kop.NumManaKinds]uint8) {
	return s.windowTapUseful(k, p, cost, cost, clamp, anyColor, false, false)
}

// windowTapRiderUseful is windowTapRider with a separate cost for the
// clamp's source usefulness: gorge's announced/legacy window filters the
// offered sources by the cast's composed cost before the convoke picks
// (paymentUsefulColours reads pc.resolvedMana), while the window itself
// closes only when the post-pick cost is payable (manaWindowAsk's
// paymentForCast). A convoke cast therefore taps exactly the sources the
// oracle's window answer would: the first source in battlefield order until
// the picks are covered, then the sources the unreduced cost still owes.
func (s *State) windowTapRiderUseful(k *prep, p uint8, cost, useful *kop.ManaCost, clamp, anyColor bool) (ok bool, held [kop.NumManaKinds]uint8) {
	return s.windowTapUseful(k, p, cost, useful, clamp, anyColor, false, false)
}

// windowTapUseful is windowTapRiderUseful with the ask's owed-cost mode:
// potential stops the window once the payer's potential mana no longer covers cost,
// as gorge funds a trigger's cost (respay.go resPayNeedsMana).
// noOwed is gorge's trigger-cost manual window, whose decision has no
// ManaPayment, so a source answers its lowest colour (windowCol).
func (s *State) windowTapUseful(k *prep, p uint8, cost, useful *kop.ManaCost, clamp, anyColor, noOwed, potential bool) (ok bool, held [kop.NumManaKinds]uint8) {
	return s.windowTapWalk(k, p, cost, useful, clamp, anyColor, noOwed, potential, false, false, false)
}

// windowTapWalk is windowTapUseful (takeDeferred is kept for the callers' signature:
// the walk takes a deferred source in its place either way). census is the
// affordability probe of gorge's unless / ward window (windowAffordsTake), whose
// offer gate lists only the sources rules unlessWindowUnits prices (windowPick);
// the window itself (payWindow) lists every usable ability. The oracle lowers every cast as a plain cast
// (Intent.Payment or the potential play's script, never Intent.Announce), so
// gorge's window is manaWindowAsk's ordinary one: no ManaPayment, no auto-fill
// option, and its answer (payexec.WindowAnswer) is the first "activate" option
// listed -- a planner-deferred source (planDeferred) in its battlefield place
// like any other, whatever the planner census affords. An Intent.Announce
// cast's announced window (announcedManaWindowAsk) would offer the auto-fill
// plan the census covers; no seat sends it here, so the walk never skips.
// payAsk is the one walk whose ask carries the ManaPayment (the announced X,
// xWindowPay): gorge's owedAbilitiesFirst reorders its listing by what the
// cost still owes, so windowFiltOwedPick takes a filter the clamp kept.
func (s *State) windowTapWalk(k *prep, p uint8, cost, useful *kop.ManaCost, clamp, anyColor, noOwed, potential, takeDeferred, census, payAsk bool) (ok bool, held [kop.NumManaKinds]uint8) {
	if s.buildLive() && s.bld().manaNP == 0 && s.bld().bfl&bflPushed != 0 && s.bld().e.kind == seSpell {
		// the payment's mana abilities queue from here: a reversal (unpushSpell) keeps what they
		// queued (CR 733.1), whichever window path reverses the cast (Spree's, a hybrid pip's)
		s.bld().manaNP = 1 + uint8(s.npend)
	}
	for !s.windowSpellSettled(k, p, cost, anyColor) {
		if potential {
			var pot avail
			other := payDesc{class: pdOther, subBound: s.g.winMode == winTrig} // a trigger's own cost reads the trigger pay gate's bound (mana_repeatpotential.go)
			s.availPotentialFor(k, p, &pot, &other, false)
			s.availSacAny(k, p, &pot, &other) // a sacrifice-any source's fixed production, once (trigCostPotentialPays)
			if !pot.affords(cost, pipMaskOf(cost), 0) && !s.potentialDynOpen(k, p, cost, &other) {
				return false, held // a dynamic-amount source's potentialUnbounded fold (mana_dyntrigcost.go)
			}
		}
		var order [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), order[:], 0)
		// gorge's manual window lists every untapped source in battlefield order and its
		// answer is the first listed (payexec.WindowAnswer): a planner-deferred source (a land
		// under Contaminated Ground's Taps watcher) is taken in its place, whoever calls
		sl, gap := s.windowPick(k, p, order[:n], useful, clamp, anyColor, s.deferLive(k), potential, census)
		if sl == NoSlot {
			if !census && s.windowHandMana(k, p, useful, clamp, anyColor) {
				continue // a hand guide, listed after every battlefield source (mana_handguide.go); the unless census prices none (unlessWindowUnits)
			}
			return false, held
		}
		if gap {
			// the gap side is windowGapArm: each arm answers the gap in the source's
			// own way and the walk goes on; only a posed tapX pick stops it
			if ok, stop := s.windowGapArm(k, p, sl, cost, useful, clamp, anyColor, noOwed, census, &held); stop {
				return ok, held
			}
			continue
		}
		if clamp && !anyColor && k.gen != nil && k.gen.anyRestrSrc && s.windowRestrOwedTap(k, p, sl, useful, noOwed) {
			continue
		}
		if s.windowHybFiltArm(k, p, sl, useful, noOwed, clamp, anyColor, potential) {
			continue // a hybrid-fee filter's lowest Combo answer (mana_paywinanswer.go)
		}
		if payAsk && clamp && !anyColor && s.windowFiltOwedPick(k, p, sl, useful) {
			// the clamp kept the source for a filter beside a plain ability: the
			// ask carries the ManaPayment, so gorge's owedAbilitiesFirst lists
			// the owed-colour abilities first (mana_filter.go
			// windowFiltOwedPick) and the filter's colour ask answers what the
			// cost still owes. A window whose ask carries no ManaPayment keeps
			// the listing order and takes the plain tap (payWinHybFilt's payWin
			// gate, windowFiltFirstFallback's note: the hybrid-only activation,
			// convoke, Spree, a lowered or free cast, waterbend, pain)
			s.windowFiltFirst(k, p, sl, useful, false)
			continue
		}
		own := &k.cards[s.cardAt(sl)]
		pc := s.windowGrantPC(k, sl)
		if k.gen != nil && k.gen.castTapOn && s.buildLive() && s.bld().e.kind == seSpell && own.castSrc != 0 {
			s.noteTapSrc(k, own, sl)
			if own.castSrc&csObserved != 0 && s.bld().obsWin&obsWinPaying == 0 {
				s.bld().obsWin |= obsWinTapped // a pre-payment tap; one by the payment's own window spends its mana
			}
		}
		col := s.windowWalkCol(k, p, cost, useful, sl, pc, clamp, anyColor, noOwed, potential)
		pc = s.windowGrantAnswer(k, sl, col, pc) // the answer is a colour only a granted ability makes: the tap is the grant's (mana_grantcost.go)
		sacOnly := windowSacOnly(k, pc)          // printed or granted (mana_grantcost.go)
		noTap := windowNoTapPain(k, pc)          // a life-only pain unit: the pain arm below pays its life (mana_pain.go)
		if k.gen != nil && k.gen.anyRestrSrc && s.windowRestrDyn(k, p, cost, sl) {
			continue
		}
		s.windowTapCost(k, sl, sacOnly || noTap)
		life := s.windowGrantLife(k, sl, pc, col)
		s.poolAdd(p, col, int(s.windowGrantYield(k, sl, pc, col)))
		s.addExtraCol(k, p, sl, pc, col)
		s.artTag(k, p, sl, col)
		if life > 0 {
			s.loseLife(k, SeatIx(p), int(life)) // a granted ability's life cost, not damage (mana_grantcost.go)
		}
		if k.painMana && pc.painCol != 0 && (pc.painOnly || uint8(pc.manaCol)&(1<<col) == 0) {
			s.painDamage(k, sl, p, int(pc.painN)) // the painful ability's rider (mana_pain.go)
		}
		if pc.manaSac || sacOnly {
			o := s.perms[sl].obj
			s.fireSac(k, sl)
			if s.manaSacParks() && s.park(k, o, parkLeave, parkManaHold(col, pc.manaN)) {
				s.pl[p].pool[col] -= pc.manaN // it floats once the move has its answer (CR 605.3b)
				return true, held
			}
			s.leave(k, sl, kop.ZoneGraveyard, false)
			// a token sacrificed to pay a spell waits in the graveyard for the
			// state-based check after the cast (gorge's Submit checks only with no
			// cast pending, so a later choice of the payment still sees it there);
			// likewise one sacrificed mid-resolution or by a planned activation's
			// mana step (bflManaStep). Another cast's token ceases at once (CR 704.3, 704.5d)
			if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard && s.buildLive() && (s.ctl != ctlBuild || s.bld().bfl&bflManaStep == 0) && s.bld().e.kind != seSpell {
				s.ceaseManaToken(o)
			}
		}
	}
	return true, held
}

// windowGapArm is windowTapWalk's gap side (a window gap the census prices, windowPick's
// gap return): each arm answers the gap in the source's own way and the walk goes on;
// only a posed tapX pick stops it (a cast pauses, not fails). The source is the first
// usable one in battlefield order, and its answer is the first ability the window lists
// (gorge's payexec.WindowAnswer), whatever the planner census affords.
func (s *State) windowGapArm(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, anyColor, noOwed, census bool, held *[kop.NumManaKinds]uint8) (ok, stop bool) {
	if s.windowLifeCtrGapTap(k, p, sl, cost, census) {
		return false, false
	}
	if a, ok := s.riderAbilityOf(k, sl); ok && (s.scriptTapsDefer(k, p).has(sl) ||
		s.riderActivatable(k, p, sl, a)) {
		ans := 0
		if !a.ManaEach && (uint8(a.ManaColors) == uint8(kop.ColAny) || a.AFlags&kop.AfManaChoice != 0) {
			pc := s.manaPCGen(k, sl)
			ans = riderAnswerBit(uint8(a.ManaColors), s.windowCol(k, p, cost, sl, pc, anyColor, noOwed))
		}
		s.riderGap(k, p, sl, ans)
		return false, false
	}
	if (s.g.winMode != winNone || s.castTapX(k)) && s.scriptTapX(k, p).has(sl) {
		if s.windowTapXGo(k, p, cost, useful, sl, clamp, anyColor, noOwed) {
			return s.g.winMode == winNone, true // the pick is posed (a cast pauses, not fails); the answer runs the walk on
		}
		return false, false
	}
	if k.gen.anyFiltFirst && k.gen.has(s.cardAt(sl), gfFilt) && s.filtFirstOnly(k, sl) {
		before := s.pl[SeatIx(p)].pool
		if noOwed && clamp {
			s.windowFiltFirstFallback(k, p, sl, useful) // mana_filterfirst.go
		} else {
			s.windowFiltFirst(k, p, sl, useful, noOwed && !clamp) // mana_filterfirst.go
		}
		// A repeatable fee-only filter (Bog Initiate's "{1}: Add {B}") neither taps nor
		// marks a use (filtUse), so an activation that pays its fee from the colour it
		// makes returns the pool as it found it and windowPick lists the same source
		// again: the walk is at a fixed point and no later pick can settle the cost.
		// The walk fails there (the filter payment search, filtPay, prices the exact plan).
		if s.filtRepeat(k, sl) && s.pl[SeatIx(p)].pool == before {
			return false, true
		}
		return false, false
	}
	if k.gen.anyComboAny && s.windowGapTap(k, p, sl, s.cardAt(sl), cost, useful, clamp, noOwed) {
		return false, false
	}
	if k.gen.anyDeferred && k.gen.has(s.cardAt(sl), gfDeferred) && k.deferIdent(s.cardAt(sl)) {
		s.windowDeferIdentTap(k, p, sl, cost, useful, clamp, noOwed) // the identity's colour ask (mana_deferident.go)
		return false, false
	}
	if k.gen.anyMultiR && s.scriptWide(k, p).has(sl) {
		s.windowWideTap(k, p, sl, cost, useful, clamp, noOwed, held)
		return false, false
	}
	if s.anyDifferentAt(k, sl) {
		s.windowAnyDifferentTap(k, p, sl, cost, useful, clamp, noOwed)
		return false, false
	}
	if s.windowGapAnyColour(k, p, sl, cost, useful, clamp, noOwed) {
		return false, false
	}
	s.tapWindowGap(k, p, sl, held)
	return false, false
}

// windowOwedCost is the cost a window source's colour ask measures against the pool
// (windowCol's owed colours, for every window walk: the paying one, windowDry's and
// a tap-another source's): cost less the floating restricted mana the payment may
// spend. That mana sits beside the pool, not in it, but gorge's colour ask prices the
// pool AvailableFor holds, restricted batches included, so Interdimensional Web Watch's
// floating {W}{W} already covers a {G/W}{G/W} cast's {W} pip and a Treasure answers {G},
// not the lowest colour (CR 106.6). buf holds the owed cost in both cases (no admitted
// restricted mana: the cost itself), so a caller may read either the return or the buffer.
func (s *State) windowOwedCost(k *prep, p uint8, cost, buf *kop.ManaCost) *kop.ManaCost {
	*buf = *cost
	if m := s.restrPoolAdmits(k, p); m != 0 {
		*buf = s.restrShortAdm(p, m, *cost)
	}
	return buf
}

// windowSpellSettled: the pool settles cost, counting the floating restricted mana
// the spell being built may spend (gorge's window prices AvailableFor's pool, CR 106.6).
func (s *State) windowSpellSettled(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) bool {
	if m := s.restrPoolAdmits(k, p); m != 0 {
		c := s.restrShortAdm(p, m, *cost)
		return s.windowPoolPays(p, &c, anyColor)
	}
	return s.windowPoolPays(p, cost, anyColor)
}

// restrPoolAdmits: the payment being made (payDescNow) may spend p's floating
// restricted mana: a spell its restriction matches, an activated ability or a
// non-spell payment (an unless or ward cost) that the restriction admits (CR 106.6).
//
//go:noinline
func (s *State) restrPoolAdmits(k *prep, p uint8) uint8 {
	if !s.rbAny(p) {
		return 0
	}
	d := s.payDescNow(NoObj)
	switch d.class {
	case pdSpell:
		return s.restrSpendable(k, p, d.card, d.og)
	case pdAct:
		return s.restrPoolAct(k, p, &d)
	case pdSpecial:
		return s.restrPoolSpecial(k, p, d.sp, d.tgt)
	}
	return s.restrPoolNonSpell(k, p)
}

// windowClampSkip: the clamp reads the colours of each ability the source
// offers (gorge paymentSourceUseful): a counter-cost ability's colour keeps
// the source listed (windowCtrOffer), though the window's answer still takes
// the source's plain first one (windowCol). Reports whether windowPick may
// skip the source without offering it.
func (s *State) windowClampSkip(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	return decided && (s.windowSrcMask(k, o)|s.windowEachMask(k, o))&want == 0 && !s.windowFiltUseful(k, p, o, want) &&
		!s.windowRestrUseful(k, o, want) && !s.windowCtrOffer(k, p, o, decided, want) && !s.windowTapXSecondUseful(k, p, o, want)
}

// windowPick is the window's next source among order (p's battlefield, in
// order): the first usable one, or a window gap (gap true); NoSlot when
// none is left. A planner-deferred source is skipped unless takeDeferred
// (windowTapWalk), but a bounded rider source's own activation (Hickory
// Woodlot's "{T}, remove a depletion counter: Add {G}{G}") is an ordinary
// window pick in a manual window: gorge's window lists every usable mana
// ability in its battlefield place and its answer takes the first listed,
// so the walk takes it here and riderGap pays its counter, mana and rider
// (mana_rider.go) in the paying cast's window. A ward or unless window's
// offer gate (census: windowAffordsTake) counts only rules unlessWindowUnits'
// sources, so there a rider whose cost or production the census does not
// price (Gemstone Mine's any colour) cannot make the pay affordable
// (riderWindowable); a fixed depletion fee (Remote Farm) can. The
// potential route (a trigger-cost window, windowTapNoOwed) takes it too:
// gorge's trigger-cost ask prices the payer's potential mana, whose walk
// counts the rider's activation, so the elected pay funds it here the same
// way (mana_rider.go).
func (s *State) windowPick(k *prep, p uint8, order []Slot, useful *kop.ManaCost, clamp, anyColor, takeDeferred, potential, census bool) (sl Slot, gap bool) {
	want, decided := uint8(0), false
	var tapsDef slotMask
	if takeDeferred {
		tapsDef = s.windowTapsDefer(k, p, census)
	}
	if clamp && !anyColor {
		want, decided = s.windowUseful(k, p, useful)
	}
	for _, o := range order {
		if s.buildLive() && s.bld().pickN != 0 && s.waterbendPicked(s.perms[o].obj) {
			continue // a waterbend pick cannot also pay the mana (gorge convokeCommitted)
		}
		if hit, skip := s.windowRiderPick(k, p, o, decided, want, census); skip {
			continue // the clamp reads colours the cost still owes
		} else if hit {
			return o, true
		}
		if census && s.ctrCensusOffer(k, p, o) {
			return o, true // the unless census's fixed counter-fee source (ctrCensusOffer)
		}
		if s.windowSourceUsable(k, o, p) {
			if !takeDeferred && s.planDeferred(k, p, o) && !s.blgIntrinsicUnit(k, o) {
				continue // the window's answer follows the planner's tier: gorge's bot never picks a deferred source (mana_tapsdefer.go); a granted basic land type's intrinsic unit is the exception (blgIntrinsicUnit)
			}
			if s.windowClampSkip(k, p, o, decided, want) {
				continue
			}
			return o, false
		}
		if (s.g.winMode != winNone || s.castTapX(k)) && s.windowTapXOffer(k, p, o, decided, want) {
			return o, true // a tap-another source (mana_windowtapx.go)
		}
		if s.windowCtrOfferFor(k, p, o, decided, want, census) {
			return o, true // a counter-cost source (mana_countercost.go)
		}
		if tapsDef.has(o) {
			if s.tapsDefRider(k, o) {
				return o, true // a bounded rider source: its own cost, mana and rider (mana_rider.go)
			}
		}
		if k.gen != nil && k.gen.anyRestrSrc {
			if m := s.windowRestrDynMask(k, p, o); m != 0 && (!decided || m&want != 0) {
				return o, false
			}
		}
		if s.windowLifeGapListed(k, p, o, decided, want) {
			return o, true // a life-only source: its life pays, its mana floats (mana_lifegap.go)
		}
		if s.windowGapListed(k, p, o) {
			c := s.cardAt(o)
			if decided && k.gen.flags[c]&gfRestr != 0 && uint8(k.t.Cards[c].Abilities[k.gen.restr(c)].ManaColors)&want == 0 {
				continue // the clamp reads a fixed colour; a Tender's has none and stays
			}
			if decided && k.gen.flags[c]&gfDeferred != 0 && s.deferMask(k, o)&want == 0 {
				// a deferred source's colours (Gilanra's {G}) the cost no longer
				// owes: gorge's window lists only the useful sources
				// (paymentSourceUseful), so a black-only X (Crypt Rats) never
				// taps it
				continue
			}
			return o, true
		}
		if k.gen != nil && k.gen.anyFiltFirst && s.windowFiltFirstUsable(k, p, o) && (!decided || s.windowFiltUseful(k, p, o, want)) && (!clamp && !DebugWinFeeStallsAlways || !s.windowFeeStalls(k, p, o, useful, anyColor)) {
			return o, true
		}
		if k.gen != nil && k.gen.anyMultiR && (!decided || want != 0) && s.windowWide(k, p, o) {
			return o, true // a wide any-colour restricted source (windowWideTap)
		}
	}
	return NoSlot, false
}

// windowSourceUsable: the manual window's pick may take sl now — a tap, a
// no-tap cost (a sacrifice-only source, or a sole life-only pain unit whose
// guard is gorge's CR 119.4 lethal one), or a live granted one.
func (s *State) windowSourceUsable(k *prep, o Slot, p uint8) bool {
	return s.windowCanTap(k, o) || s.windowSacNow(k, o) || k.painMana && s.canLifeOnlyMana(k, o, p)
}

// windowTapCost: the window's payment taps sl unless the ability pays without
// a tap — a sacrifice-only source (the walk sacrifices it) or a life-only
// pain one (the pain arm below pays its life). It reports whether it tapped.
func (s *State) windowTapCost(k *prep, sl Slot, noTap bool) bool {
	if noTap {
		return false
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if k.gen != nil && k.gen.anyManaRider && s.manaPCGen(k, sl).manaIntrinsic {
		// the view is the granted types' intrinsic unit (blgView): manaTapped
		// ran no rider for it (trig_taps.go), but the window's answer is the
		// card's own ability -- gorge lists printed abilities before the
		// appended intrinsics and takes the first (payexec.WindowAnswer), so
		// the rider resolves with the tap (CR 605.3b)
		s.manaRider(k, sl)
	}
	return true
}

// windowRiderPick: o is a bounded rider source gorge's window lists (hit); skip
// when the clamp keeps none of its colours.
func (s *State) windowRiderPick(k *prep, p uint8, o Slot, decided bool, want uint8, census bool) (hit, skip bool) {
	if k.gen == nil || !k.gen.anyManaRider {
		return false, false
	}
	c := s.cardAt(o)
	a, ok := s.riderAbilityOf(k, o)
	if !ok || !(int(k.cards[c].manaAb) == int(k.riderAbOf(c)) || s.riderPlainWindow(k, o, a)) || !s.riderWindowable(k, p, o, a, census) {
		return false, false
	}
	mask := uint8(a.ManaColors)
	if a.ManaEach {
		mask = uint8(kop.ColAny)
	}
	if decided && mask&want == 0 {
		return false, true
	}
	return true, false
}

// windowCanTap is canTapForMana for the window's walk: a source whose only
// usable mana ability is one effects granted it, and whose grant the payment
// being built cannot spend, is not offered (CR 106.6), the same gate windowGap
// puts on a printed restricted source. A source with an admitted grant, or a
// usable printed ability, is.
func (s *State) windowCanTap(k *prep, sl Slot) bool {
	var pc *pcard
	if k.gen == nil {
		pc = &k.cards[s.cardAt(sl)]
	} else {
		pc = s.windowGrantPC(k, sl)
	}
	if pc.painOnly && pc.painLife && int(s.pl[SeatIx(s.perms[sl].ctrl)].life) <= int(pc.painN) && s.g.winMode != winTrig {
		// A trigger's own cost window (winTrig) keeps the bare CR 119.4 test:
		// gorge lists the source at exactly its life cost and the payer dies
		// paying it (Mana Confluence at 1 life, Genesis's {2}{G}).
		// The unless / ward offer gate prices a paid-life source through
		// gorge's paid-cost census (painAvailMaskAt; gorge's paidCostShape),
		// which counts it only while its activation leaves the payer alive.
		// canTapForManaPC keeps the bare CR 119.4 legality test (life >= the
		// payment), so at exactly the cost the source is tappable there; the
		// walk must not take what the gate priced out (an unless {2} funded by
		// Phyrexian Lens at 1 life: gorge offers decline only).
		return false
	}
	if k.gen == nil {
		return s.canTapForMana(k, sl)
	}
	return s.canTapForManaPC(k, sl, pc)
}

// windowGrantPC is the view of the mana ability a window activation of sl
// takes: the source's own printed ability, or the first granted one the
// payment being built may spend (grantsAdmitted). &noManaPC when every
// granted ability refuses the payment.
func (s *State) windowGrantPC(k *prep, sl Slot) *pcard {
	if k.gen == nil {
		return &k.cards[s.cardAt(sl)]
	}
	if k.gen.anyCantAct && s.actForbidden(k, sl, true) {
		return &noManaPC // a CantBeActivated lock, a granted ability's too (cast_cantactivate.go)
	}
	if s.g.winMode == winTrig {
		// a trigger's own cost window lists a Produced$ ColorIdentity source a
		// rider or a cost beyond {T} kept out of the planner's census: it makes
		// one mana of a colour in its controller's commander identity (CR
		// 903.4), the shape gorge's potential-bound offer prices but the
		// planner lists no unit for (legal.go identityPotentialView)
		if pc, ok := s.identityPotentialView(k, sl); ok {
			return pc
		}
	}
	if len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || s.ownManaUsable(k, sl) {
		// the window lists the card's own ability: a rider source whose view
		// is a granted basic land type's intrinsic unit reads its rider
		// (blgGapPC, CR 605.3b)
		return s.blgGapPC(k, sl)
	}
	var gi [maxGrants]int16
	n := s.grantIdxs(k, sl, &gi)
	if n == 0 {
		return s.blgGapPC(k, sl) // the card's own ability, the rider of an intrinsic-unit source (blgGapPC)
	}
	d := s.payDescNow(NoObj)
	if n = s.grantsAdmitted(k, sl, &gi, n, &d); n == 0 {
		return &noManaPC
	}
	return s.windowPayableGrant(k, sl, &gi, n)
}

// windowWide: sl is p's untapped wide any-colour restricted source (Flamebraider)
// whose mana the payment being built may spend (CR 106.6): an activate option of
// gorge's cast window, though its census mana is a gap, not a planner unit.
func (s *State) windowWide(k *prep, p uint8, sl Slot) bool {
	if !s.scriptWide(k, p).has(sl) {
		return false
	}
	d := s.payDescNow(NoObj)
	return s.restrSrcAdmits(k, p, sl, &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))], &d)
}

// windowWideTap activates wide source sl in the window and floats its two mana
// beside the pool (restricted, State.rb). The ask lists the colours the clamp keeps
// (paymentUsefulColours) once per mana, and each mana answers a colour the cost
// still owes, else the lowest listed (payexec.WindowChoices), the owed amount
// falling by what the first mana covers; a window whose ask carries no cost
// (noOwed, windowCol) owes nothing, so each mana is the lowest listed. Mana the
// byte cannot hold as restricted floats plain (held).
func (s *State) windowWideTap(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool, held *[kop.NumManaKinds]uint8) {
	ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))]
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	mask := uint8(kop.ColAny)
	if clamp {
		if want, decided := s.windowUseful(k, p, useful); decided && want&mask != 0 {
			mask &= want
		}
	}
	var owed [len(cost.Pips)]uint8
	if !noOwed {
		owed = cost.Pips
		for c := range owed {
			owed[c] -= min(owed[c], s.pl[p].pool[c])
		}
	}
	for i := 0; i < 2; i++ {
		col := bits.TrailingZeros8(mask)
		for c := 0; c < 5; c++ {
			if mask&(1<<c) != 0 && owed[c] > 0 {
				col = c
				owed[c]--
				break
			}
		}
		if !s.floatRestrAb(k, p, sl, ab, col, 1) {
			held[col]++
		}
	}
}

// colourAskOrder writes mask's colours in the order gorge's askManaColor
// offers a mana ability's colour choice: a printed Combo's printed order
// (kop.Ability.ManaColOrd through manaColourOrder: the TDM Devotees,
// Opulent Palace's "Combo B G U"), then any other bit in W U B R G C order.
// ab is the ability being activated (nil when its shape was not read); its
// printed order counts only when it names the whole mask, because a granted
// ability's colours follow the source's own in gorge's list.
func colourAskOrder(ab *kop.Ability, mask uint8, out *[kop.NumManaKinds]uint8) (int, bool) {
	if ab == nil || ab.ManaColOrd == 0 || uint8(ab.ManaColors)&0x1f != mask&0x1f {
		return 0, false // the common W U B R G C order: the caller's own loop
	}
	return printedAskOrder(ab, out), true
}

// printedAskOrder is colourAskOrder's printed list, kept out of line so the
// common unprinted guard inlines at its callers.
func printedAskOrder(ab *kop.Ability, out *[kop.NumManaKinds]uint8) int {
	var ord [5]uint8
	return copy(out[:], ord[:manaColourOrder(ab, &ord)])
}

// windowDemand is windowUseful's fixed-pip shortfall on its own census, the
// restriction-adjusted pool (CR 106.6), without the verdict. A hybrid pip
// leaves the verdict unpriced (windowUseful bails) while gorge's
// owedColoursFirst still lists the shortfall first (CR 601.2g), so the window
// answer reads the census directly there.
func (s *State) windowDemand(k *prep, p uint8, cost *kop.ManaCost) uint8 {
	pool := &s.pl[p].pool
	if m := s.restrPoolAdmits(k, p); m != 0 {
		var t [kop.NumManaKinds]uint8
		t = *pool
		s.rbUnits(p, m, &t)
		pool = &t
	}
	var want uint8
	for c := 0; c < kop.NumManaKinds; c++ {
		if cost.Pips[c] > pool[c] {
			want |= 1 << c
		}
	}
	return want
}

// askOwedColour is the first colour of mask the demand want still owes, in the
// ask order the ability lists (a printed Combo's printed order, else W U B R
// G): gorge's ask lists what a fixed pip still owes first (owedColoursFirst,
// CR 601.2g). -1 when the demand names none of mask: the caller keeps its
// lowest then.
func askOwedColour(ab *kop.Ability, mask, want uint8) int {
	m := mask & want
	if m == 0 {
		return -1
	}
	if m&(m-1) == 0 && m < 1<<5 {
		return bits.TrailingZeros8(m) // one owed colour: every ask order lists it
	}
	var order [kop.NumManaKinds]uint8
	n, printed := colourAskOrder(ab, mask, &order)
	if !printed {
		return int(bits.TrailingZeros8(m))
	}
	for i := 0; i < n; i++ {
		if c := order[i]; c < 5 && m&(1<<uint(c)) != 0 {
			return int(c)
		}
	}
	return -1 // colourAskOrder listed mask's colours; none of them is owed
}

// windowCol is the colour the window's source sl (mana facts pc) makes: a
// colour the cost still owes, else the lowest (the oracle's window answer,
// payWindowAnswer). A multi-ability source owing nothing: its first ability, the
// lowest colour (mana_multicolour.go). noOwed is a window whose ask carries no cost:
// there is nothing to owe, so the source takes its lowest colour, which for
// a plain any-colour source is W (gorge's trigger-cost manual window, the
// payexec.WindowAnswer fallback). When a pip is still owed, the answer is the
// first owed colour of the source's ask order: for a printed Combo that is its
// printed order (colourAskOrder), not W U B R G. The owed colours are measured
// against cost less the floating restricted mana the payment may spend
// (windowOwedCost); Starting Town's clamp reads the raw cost, as windowUseful
// adds that mana itself.
func (s *State) windowCol(k *prep, p uint8, raw *kop.ManaCost, sl Slot, pc *pcard, anyColor, noOwed bool) int {
	var owed kop.ManaCost
	s.windowOwedCost(k, p, raw, &owed)
	return s.windowColOwed(k, p, raw, &owed, sl, pc, anyColor, noOwed)
}

// windowFallbackCol is the colour a window source makes when no owed colour
// picks one: the lowest of mask, except a fixed first ability beside a
// chosen-colour twin (Quirion Elves), whose ability 0 is the fixed colour even
// when the chosen colour is lower.
func windowFallbackCol(pc *pcard, mask uint8) int {
	if pc.manaFixedChosen {
		if m := srcMask(pc); m != 0 {
			return bits.TrailingZeros8(m)
		}
	}
	return bits.TrailingZeros8(mask)
}

// windowColOwed is windowCol for a cost whose shortfall the pool is measured against as owed:
// cost less the restricted mana the payment may spend (windowOwedCost). cost is the full
// (raw) cost: the helpers that price the restricted pool themselves (windowDemand,
// windowUseful) read it, so the mana is not taken off it twice, while owed is the shortfall.
func (s *State) windowColOwed(k *prep, p uint8, cost, owed *kop.ManaCost, sl Slot, pc *pcard, anyColor, noOwed bool) int {
	mask := s.windowOwnMask(k, sl)
	col := windowFallbackCol(pc, mask)
	if anyColor {
		col = s.windowAnyColour(p, owed, mask, pc.manaMulti)
	} else if !noOwed {
		if pc.manaMulti && k.gen != nil && k.gen.anyVergeOrder {
			if c0, ok := s.vergeAbility0Col(k, sl); ok && c0 < 5 && owed.Pips[c0] > s.pl[p].pool[c0] {
				return c0 // a Verge's ability 0 comes before its lower colour
			}
		}
		all := mask // windowSrcMask on the own mask already read: a granted ability's colour a pip owes counts too
		if k.gen != nil && len(k.gen.grantPC) > 0 && s.grantMay(k, sl) {
			all |= s.windowGrantMask(k, sl)
		}
		if !pc.manaMulti && !pc.manaChosen && pc.manaCol != kop.ColAny && pc.painCol == 0 && bits.OnesCount8(mask&0x1f) > 1 {
			// a printed Combo is flattened into one option per colour, each with
			// its symbol, and the window answers the first such option an owed
			// colour matches, else the lowest: a granted ability's option has no
			// symbol and is never taken beside them (Misty Palms Oasis under a
			// Great Divide Guide owing {R} answers {W})
			all = mask
			if pc.manaAb >= 0 && pc == &k.cards[s.cardAt(sl)] {
				// the options are the ability's own token order
				// (effects.ComboColours, kop.Ability.ManaColOrd), not WUBRG:
				// Thornwood Falls' Combo G U answers {G} before {U}
				ab := &k.t.Cards[s.cardAt(sl)].Abilities[pc.manaAb]
				var ord [5]uint8
				for _, c := range ord[:manaColourOrder(ab, &ord)] {
					if all&(1<<c) != 0 && owed.Pips[c] > s.pl[p].pool[c] {
						return int(c)
					}
				}
				return col
			}
		}
		// gorge's owedColoursFirst keeps the ask order, so the first owed
		// colour is the first of the source's own ask order (a Combo's
		// printed order), not the lowest WUBRG index.
		// Only the source's own plain profile carries a card-local ability
		// index: a grant/var/gate view's manaAb indexes the granting card's
		// abilities (genPrep.grantPC), so reading it here would panic or name
		// another card.
		// A source of one own colour lists it in every order: only a wider
		// one reads its printed order.
		var seen uint8
		if own := mask & 0x1f; own&(own-1) != 0 && pc.manaAb >= 0 {
			if cd := s.cardAt(sl); pc == &k.cards[cd] {
				var order [kop.NumManaKinds]uint8
				nOrder, printed := colourAskOrder(&k.t.Cards[cd].Abilities[pc.manaAb], mask, &order)
				for i := 0; i < nOrder; i++ {
					c := int(order[i])
					if c < 5 && all&(1<<uint(c)) != 0 && owed.Pips[c] > s.pl[p].pool[c] {
						return c // gorge lists what a fixed pip still owes first (owedColoursFirst, owedAbilitiesFirst)
					}
				}
				if printed {
					seen = mask
				}
			}
		}
		if pc.blgFirst != 0 {
			// a granted basic land type's view of the card's own Combo: its
			// printed token order comes first, the intrinsic colours after
			if ab := manaAbView(&k.t.Cards[s.cardAt(sl)], pc); ab != nil && ab.Kind == kop.AbMana && ab.AFlags&kop.AfManaChoice != 0 {
				var ord [5]uint8
				for _, c := range ord[:manaColourOrder(ab, &ord)] {
					if all&(1<<c) != 0 && owed.Pips[c] > s.pl[p].pool[c] {
						return int(c)
					}
				}
			}
		}
		if pc.blgFirst != 0 {
			// a basic under a granted type lists its own colour before the granted ones
			// (chars.appendLandTypes, owedAbilitiesFirst): an Island owing {W}{U} answers {U}
			own := uint8(k.cards[s.cardAt(sl)].manaCol) & 0x1f & uint8(all)
			if f := uint(pc.blgFirst - 1); f < 5 && own&(1<<f) != 0 && owed.Pips[f] > s.pl[p].pool[f] {
				return int(f)
			}
			for c := 0; c < 5; c++ {
				if own&(1<<c) != 0 && owed.Pips[c] > s.pl[p].pool[c] {
					return c
				}
			}
		}
		for c := 0; c < 5; c++ {
			if all&(1<<c) != 0 && seen&(1<<c) == 0 && owed.Pips[c] > s.pl[p].pool[c] {
				return c // a granted ability's colour follows the source's own
			}
		}
	}
	if pc.blgFirst != 0 {
		col = int(pc.blgFirst - 1) // a granted basic land type: the printed ability first (static_landtype.go)
	}
	if pc.manaMulti && k.gen != nil && k.gen.anyVergeOrder {
		if c0, ok := s.vergeAbility0Col(k, sl); ok {
			col = c0 // a Verge's window activates its ability 0 (mana_gate.go)
		}
	}
	if k.painMana && pc.painCol != 0 && !pc.painOnly && !anyColor {
		// A painful source's ask: the pain arm answers it (windowColPain,
		// mana_pain.go), the lowest colour the ask still lists.
		return s.windowColPain(k, p, cost, pc, mask, noOwed)
	}
	return col
}

// windowDry runs windowTapRider's source walk without its effects: only the
// pool, the tapped bits, the floating restricted batches and the artifact pool
// tag move, and all are restored before it returns.
// ok is whether the window pays cost; known is false when the walk reaches
// a source whose activation does more than tap and add mana (a window gap
// other than a first-only filter, a sacrificed source, a source that
// sacrifices without tapping), which the caller must then probe another way.
// A Treasure prices like any tapped source: gorge's census counts it as a unit
// (castWindowPays).
func (s *State) windowDry(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) (ok, known bool) {
	ok, known, _ = s.windowDryRestr(k, p, cost, anyColor)
	return ok, known
}

// windowDryRestr is windowDry, and also reports whether the walk stopped
// unknown only because a source's restricted ability is owed ahead of its
// census one (windowRestrOwed): mana gorge's census prices as a typed unit
// (castWindowReachable), which the real walk on a fork can count.
func (s *State) windowDryRestr(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) (ok, known, restr bool) {
	ok, known, restr, _ = s.windowDryWalk(k, p, cost, anyColor)
	return ok, known, restr
}

// windowDryWalk is windowDryRestr, and also reports whether the walk stopped
// unknown only at a source that pays without a tap (sacStop): a last-resort
// unit the payment planner prices where this walk does not — a sacrifice-only
// source (an Eldrazi Spawn), or a life-only pain one (a Treasonous Ogre,
// mana_pain.go).
func (s *State) windowDryWalk(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) (ok, known, restr, sacStop bool) {
	pool := s.pl[p].pool
	// A filter's fee is spent by the real fee arm (windowDryFilter ->
	// filtFeeSpend), which carves the floating restricted batches and their
	// artifact tag as the payment does; the walk restores both with the pool.
	rb := s.rb[SeatIx(p)]
	art := s.g.poolArt[SeatIx(p)]
	var tapped [MaxPerm]Slot
	var flags [MaxPerm]uint16
	var used [MaxPerm]uint8
	nt := 0
	ok, known = true, true
	// The walk settles on the pool the payment may spend: floating restricted
	// mana the spell admits counts, as windowSpellSettled reads it (CR 106.6).
	// A plain-pool census here would tap a source for a pip the restricted
	// mana already covers, after windowPick's usefulness clamp (which does
	// count it) offered none.
	for !s.windowSpellSettled(k, p, cost, anyColor) {
		var order [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), order[:], 0)
		sl, gap := s.windowPick(k, p, order[:n], cost, true, anyColor, s.deferLive(k), false, false) // the real walk's pick (windowTapWalk): a deferred source in its place
		if sl == NoSlot {
			ok = false
			break
		}
		if gap && s.windowFiltFirstUsable(k, p, sl) {
			tapped[nt], flags[nt], used[nt] = sl, s.perms[sl].flags, s.perms[sl].actUsed
			nt++
			s.windowDryFilter(k, p, cost, sl)
			continue
		}
		if !anyColor && !gap {
			// the real walk may take a restricted ability here ahead of the
			// census one (windowRestrOwedTap): its mana floats restricted in
			// State.rb, which this plain-pool census cannot model. Leave the
			// walk to the caller's full probe.
			if _, _, owed := s.windowRestrOwed(k, p, sl, cost); owed {
				ok, known, restr = false, false, true
				break
			}
		}
		pc := s.windowGrantPC(k, sl)
		dyn := k.gen != nil && k.gen.anyRestrSrc && s.windowRestrDynMask(k, p, sl) != 0
		if gap && !s.dryDeferPriced(k, sl, pc) ||
			windowSacOnly(k, pc) || windowNoTapPain(k, pc) || s.perms[sl].flags&pfTapped != 0 || dyn {
			ok, known = false, false
			sacStop = !gap && !dyn && (windowSacOnly(k, pc) || windowNoTapPain(k, pc)) && s.perms[sl].flags&pfTapped == 0
			break
		}
		col := s.windowCol(k, p, cost, sl, pc, anyColor, false)
		tapped[nt], flags[nt], used[nt] = sl, s.perms[sl].flags, s.perms[sl].actUsed
		nt++
		s.perms[sl].flags |= pfTapped
		s.poolAdd(p, col, int(pc.manaN))
		s.addExtraCol(k, p, sl, pc, col)
	}
	for i := 0; i < nt; i++ {
		s.perms[tapped[i]].flags, s.perms[tapped[i]].actUsed = flags[i], used[i]
	}
	s.pl[p].pool = pool
	s.rb[SeatIx(p)] = rb
	s.g.poolArt[SeatIx(p)] = art
	return ok, known, restr, sacStop
}

// windowDryFilter is windowDry's step over a first-only filter the floating
// mana pays for (gorge's castWindowReachable, CR 605.3a): it spends the fee
// exactly as the real window's filtDo does (the pip, repeat pip or hybrid pip
// first, then filtFeeSpend's plain pool in C W U B R G order), so a fee that
// takes a pip the cost still owes leaves the dry walk short just as it leaves
// the real payment short. The filter's colour is then the one the remaining
// pool still owes (windowFiltFirst's answer).
func (s *State) windowDryFilter(k *prep, p uint8, cost *kop.ManaCost, sl Slot) {
	if s.filtRepeat(k, sl) {
		// repeatable: no use to mark (mana_filterrepeat.go)
	} else if s.filtNoTap(k, sl) {
		s.perms[sl].actUsed |= 1 << uint(k.gen.filt(s.cardAt(sl)))
	} else {
		s.perms[sl].flags |= pfTapped
	}
	pl := &s.pl[p]
	fee := s.filtFee(k, sl)
	if col, ok := s.filtPip(k, sl); ok {
		s.filtPayPip(k, p, sl, col)
		fee = 0
	} else if col, ok := s.filtRepeatPip(k, sl); ok {
		s.filtPayPip(k, p, sl, col) // a fee-only filter's pip first, then its generic part
		fee--
	} else if a, b, ok := s.filtHyb(k, sl); ok {
		s.filtPayHyb(k, p, sl, a, b) // the hybrid pip, in its first printed colour when held
		fee = 0
	}
	s.filtFeeSpend(k, p, sl, fee)
	ab := s.filtAb(k, sl)
	if ab.ManaEach {
		for c, n := range filtEachVec(ab) {
			pl.pool[c] += n // every colour at once (mana_filtereach.go)
		}
		return
	}
	mask := uint8(ab.ManaColors)
	col := bits.TrailingZeros8(mask)
	// The demand is the pool the payment may spend (windowUseful, CR 106.6):
	// a pip its restricted mana covers is not owed. A hybrid pip leaves the
	// verdict undecided (windowUseful prices nothing), so the census alone
	// gives the demand there.
	want, _ := s.windowUseful(k, p, cost)
	if cost.Hyb[0] != 0 {
		want = s.windowDemand(k, p, cost)
	}
	if a := askOwedColour(ab, mask, want); a >= 0 {
		col = a
	}
	pl.pool[col]++
}

// windowAnyColour picks the colour an any-colour window source makes: a
// colourless pip the pool still owes first, else a colour while the pool's
// coloured mana still owes a coloured pip, else {C} when the source makes
// it (spending colourless for generic keeps colours for pips), else the
// lowest colour. Gorge's window backtracks over the source's alternatives
// (cast_window.go ordered[i].Alts); a source that makes both {C} and a
// colour must not be spent on {C} while a coloured pip is unpaid.
func (s *State) windowAnyColour(p uint8, cost *kop.ManaCost, mask uint8, multi bool) int {
	if multi {
		return bits.TrailingZeros8(mask) // its first ability, the lowest colour
	}
	pl := &s.pl[p]
	needC := cost.Pips[kop.C] > pl.pool[kop.C]
	colPips, colPool := 0, 0
	for c := 0; c < 5; c++ {
		colPips += int(cost.Pips[c])
		colPool += int(pl.pool[c])
	}
	if needC && mask&(1<<kop.C) != 0 {
		return kop.C
	}
	if colPips > colPool {
		if m := mask & 0x1f; m != 0 {
			return bits.TrailingZeros8(m)
		}
	}
	if mask&(1<<kop.C) != 0 {
		return kop.C
	}
	return bits.TrailingZeros8(mask)
}

// windowPoolPays is poolPays under the may-play any-colour rider.
func (s *State) windowPoolPays(p uint8, cost *kop.ManaCost, anyColor bool) bool {
	if anyColor {
		return s.poolPaysAnyColor(p, cost)
	}
	return s.poolPays(p, cost)
}

// windowGap reports whether sl is p's usable Bloom Tender or restricted
// source (Giada): no autopay source, but an activate option of gorge's
// ward / unless mana window; a restricted one only when the payment may spend its mana.
func (s *State) windowGap(k *prep, p uint8, sl Slot) bool {
	c := s.cardAt(sl)
	if k.gen.anyComboAny && (k.isComboAny(c) || k.isReflAmt(c)) {
		return s.caGaps(k, p).has(sl) // a Combo Any source: the window's own tap (windowComboTap); a reflected Amount$ one too (windowReflAmtTap, mana_reflamt.go)
	}
	if k.gen.flags[c]&(gfEachCol|gfRestr|gfDeferred) == 0 {
		return false
	}
	pm := &s.perms[sl]
	if pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) ||
		pm.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 ||
		k.gen.anyCantAct && s.actForbidden(k, sl, true) { // a CantBeActivated lock (cast_cantactivate.go), the same gate gapMask applies
		return false
	}
	if k.gen.flags[c]&gfDeferred != 0 && (!s.deferLifeOK(k, p, sl) || s.deferNoMana(k, sl)) {
		return false
	}
	if k.gen.flags[c]&(gfEachCol|gfRestr) == gfRestr {
		// CR 106.6: a restricted source whose mana the payment cannot spend is
		// not offered (gorge's window lists only the abilities the payment meets).
		// One whose evaluated amount is no mana (Nardole, Resourceful Cyborg
		// at no counters) is not offered either (gorge's paymentSourceUseful
		// skips what manaAbilityYieldsNothing reports).
		d := s.payDescNow(NoObj)
		a := &k.t.Cards[c].Abilities[k.gen.restr(c)]
		// A restricted ability is a gfRestr one only when restrTapMana read a
		// nonzero ManaColors, so a "Chosen" production without a printed
		// colour never reaches this arm; a Combo <C> Chosen one keeps its
		// fixed colour and stays listed (a copy that chose nothing still
		// makes that colour). The census read of the chosen-only shape is
		// manaPCGen's, which fails it closed.
		return s.restrSrcAdmits(k, p, sl, a, &d) && (a.AFlags&kop.AfManaDynamic == 0 || s.dynManaN(k, a, sl) > 0)
	}
	return true
}

// tapWindowGap taps window gap sl: a Tender's colours into the pool, a
// restricted source's mana beside it (State.rb, which the cost cannot spend).
// held stays empty: the restricted mana never enters the pool.
func (s *State) tapWindowGap(k *prep, p uint8, sl Slot, held *[kop.NumManaKinds]uint8) {
	var into [kop.NumManaKinds]uint8
	s.tapGap(k, p, sl, s.windowRestr(k, p).and(slotBit(sl)), &into)
}

// windowGapAnyColour taps a restricted source of several colours (Gideon's Memorial)
// for the colour gorge's ask answers: the ask lists what the clamp keeps
// (paymentUsefulColours), the answer is what an announced pip still owes, else the
// lowest listed. The mana floats beside the pool. It reports whether sl is such a source.
func (s *State) windowGapAnyColour(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, noOwed bool) bool {
	c := s.cardAt(sl)
	if k.gen.flags[c]&gfRestr == 0 || !s.windowRestr(k, p).has(sl) {
		return false
	}
	ab := &k.t.Cards[c].Abilities[k.gen.restr(c)]
	mask := uint8(ab.ManaColors)
	if bits.OnesCount8(mask) < 2 {
		return false
	}
	if clamp {
		if want, decided := s.windowUseful(k, p, useful); decided && mask&want != 0 {
			mask &= want
		}
	}
	col := bits.TrailingZeros8(mask)
	if !noOwed {
		// The clamp's narrowed mask is the answer's menu, but the ask order is
		// the ability's own: colourAskOrder needs the printed mask whole.
		var order [kop.NumManaKinds]uint8
		nOrder, printed := colourAskOrder(ab, uint8(ab.ManaColors), &order)
		if printed {
			for i := 0; i < nOrder; i++ {
				ci := int(order[i])
				if ci < 5 && mask&(1<<uint(ci)) != 0 && cost.Pips[ci] > s.pl[p].pool[ci] {
					col = ci
					break
				}
			}
		} else {
			for ci := 0; ci < 5; ci++ {
				if mask&(1<<ci) != 0 && cost.Pips[ci] > s.pl[p].pool[ci] {
					col = ci
					break
				}
			}
		}
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if !s.floatRestrAb(k, p, sl, ab, col, s.restrAbN(k, sl, ab)) {
		s.capFull(StallRestrSrc)
	}
	return true
}

// windowAffords: gorge's mana window pays cost from p's sources counting
// the Tenders' mana (availFor does not), simulated on a copy. The second
// return is the first rule-gap stall the simulated walk latched (StallNone
// none): the probe declares nothing — a caller that keeps searching may still
// price the play, and the drop site declares the stall only where every route
// failed (windowPays, payWardN, applyWardManaLife, the madness gates).
func (s *State) windowAffords(k *prep, p uint8, cost *kop.ManaCost) (bool, uint8) {
	return s.windowAffordsTake(k, p, cost, false, false)
}

// tapsDefRider: the Taps-deferred source o has a bounded rider ability the
// window may take.
func (s *State) tapsDefRider(k *prep, o Slot) bool {
	_, ok := s.riderAbilityOf(k, o)
	return ok
}

// unlessSkipsRider: a depletion land's activation is in an unless window
// when gorge's unless census prices it -- a fixed, source-anchored counter
// removal on a production needing no colour choice (Remote Farm, CR 608.2g;
// riderCensusFree). A choice-shaped one (Gemstone Mine's any colour) stays
// out, as the census leaves it out (payWindow, windowAffordsTake). A ward's
// window takes either. The potential route and the cast windows still take
// them.
func (s *State) unlessSkipsRider(a *kop.Ability) bool {
	return s.g.turnF&turnUnlessWin != 0 && a.ManaRider == kop.ManaRiderDeplete && !riderCensusFree(a)
}

// turnUnlessWin is genState.turnF's bit 15: the walk is an unless / ward
// window (unlessSkipsRider). It lives in turnF's free bits so State's fixed
// layout does not move.
const turnUnlessWin turnFT = 1 << 15

func (s *State) setUnlessWin(on bool) {
	if on {
		s.g.turnF |= turnUnlessWin
	} else {
		s.g.turnF &^= turnUnlessWin
	}
}

// riderOfferable: some source's first mana ability is a bounded rider
// activation the window walk can take (windowPick's rider arm): gorge's
// manual windows list it and the census probes must count it. census is an
// unless / ward window, whose offer gate lists the sources gorge's
// unlessWindowUnits prices (riderCensusFree): a rider whose cost or
// production the census does not price is no unit there, so it is not
// offerable.
func (s *State) riderOfferable(k *prep, p uint8, census bool) bool {
	if k.gen == nil || !k.gen.anyManaRider {
		return false
	}
	for m := s.ctrlM[SeatIx(p)].and(s.occ); m.any(); m = m.rest() {
		sl := m.first()
		if a, ok := s.riderAbilityOf(k, sl); ok && s.riderPotential(k, p, sl) && (!census || riderCensusFree(a)) {
			return true
		}
	}
	return false
}

// unlessTapsDeferred is scriptTapsDefer as an unless / ward window prices it:
// leaving out the riders riderCensusFree rejects.
func (s *State) unlessTapsDeferred(k *prep, p uint8) bool {
	for m := s.scriptTapsDefer(k, p); m.any(); m = m.rest() {
		if a, ok := s.riderAbilityOf(k, m.first()); !ok || riderCensusFree(a) {
			return true
		}
	}
	return false
}

// windowAffordsTake is windowAffords; unless: the window is an unless cost's,
// which leaves a depletion land alone when its rider is not a census unit
// (unlessSkipsRider; a ward's takes it). takeDeferred is an unless / ward
// window (payWindow), which taps a planner-deferred source (a counter-rider
// mana source, Twitching Doll, or a depletion land the census prices) once
// the rest cannot cover the cost: gorge's unlessWindowUnits lists it, so the
// pay is offered (CR 118.3, 605.3b). So does it tap a fixed counter-fee
// source the census prices (unlessCtrGaps, Weather Maker), whose activation
// the windowPick census arm takes (ctrCensusOffer). So does it take a
// first-only filter the unless census prices (scriptFilt: Apprentice
// Wizard's "{U}, {T}: Add {C}{C}{C}"), whose fee the walk pays from the
// floating mana an earlier same-window activation produced (gorge's
// unlessWindowUnits' literal-mana-fee arm and UnlessManaReachable's fee
// transition, CR 605.3b) -- without the disjunct the gate offers decline
// only where gorge's census prices the source and this walk proves the
// chain (kfuzz pair 25 of dr-mbc-baab).
func (s *State) windowAffordsTake(k *prep, p uint8, cost *kop.ManaCost, takeDeferred, unless bool) (bool, uint8) {
	s.setUnlessWin(unless)
	defer s.setUnlessWin(false)
	if k.gen == nil || !(k.gen.anyEachCol && s.scriptGaps(k, p).any() || k.gen.anyComboAny && s.caGaps(k, p).any() || s.scriptDeferred(k, p).any() || takeDeferred && s.unlessTapsDeferred(k, p) || takeDeferred && s.unlessCtrGaps(k, p) || takeDeferred && k.gen.anyFiltFirst && s.scriptFilt(k, p).any() || s.riderOfferable(k, p, takeDeferred)) {
		return false, 0
	}
	t := s.fork()
	defer s.unfork()
	was := t.pool.census
	t.pool.census = takeDeferred
	ok, _ := t.windowTapWalk(k, p, cost, cost, false, false, false, false, takeDeferred, takeDeferred, false)
	t.pool.census = was
	stall := uint8(0)
	if !ok {
		// The greedy walk missed a filter-first source's fee-funded plan: its
		// generic fee may need a funder the cost does not owe, and the funder
		// may list before the filter (Overflowing Basin beside Power Depot).
		// The filter payment search prices a fresh board exactly: the failed
		// walk's taps and floating mana must not skew the plan. It is the same
		// census walk's source set, and its latched stall is returned to the
		// caller, never declared by the probe.
		ok, stall = s.filtPay(k, p, cost, takeDeferred)
		if !ok {
			walkStall := t.g.stalled
			if walkStall != 0 {
				stall = walkStall
			}
		}
	}
	return ok, stall
}

// windowAffordsN is windowAffords for an unless cost of amt with pips.
func (s *State) windowAffordsN(k *prep, p uint8, amt int, pips int16, unless bool) (bool, uint8) {
	c := manaOf(amt, pips)
	return s.windowAffordsTake(k, p, &c, true, unless)
}

// restrTapMana: a is a plain tap mana ability with a spend restriction
// (Giada's Angel-only {W}), a window / script gap (genPrep.restr). Kept
// out of line, with the other window code, off prepareGen's hot
// neighbours' alignment.
//
//go:noinline
func restrTapMana(a *kop.Ability) bool {
	return a.ManaRestrict != 0 && a.ManaColors != 0 && a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// windowPays: execPay's unless / ward mana cost that gorge's window pays
// with a Tender's mana (windowAffords), out of line. The second return is the
// first rule-gap stall a failed probe's fork latched (StallNone none): the
// caller declares it only where every route failed (tapXScript's shape).
//
//go:noinline
func (s *State) windowPays(k *prep, p uint8, kind int, skip uint16, amt int, pips int16) (bool, uint8) {
	if kind != kop.PayMana || skip&(pfWard|pfModes) == 0 {
		return false, 0
	}
	if ok, stall := s.windowAffordsN(k, p, amt, pips, skip&pfWard == 0); ok {
		return true, stall // a found plan's spill is a capacity stall the caller declares
	} else {
		// An unless cost is no spell's payment: restricted mana whose restriction
		// admits a non-spell payment (a Powerstone's {C}, CR 106.6) pays it, as it
		// pays a ward (nonSpellRestrAffords).
		c := manaOf(amt, pips)
		if s.nonSpellRestrAffords(k, p, &c) {
			return true, 0
		}
		return false, stall
	}
}

// restrKinds is how many distinct spend restrictions a table's restricted
// mana can name (genPrep.restrF with its variant): a Standard pool's table
// holds up to six; a ninth would stall.
const restrKinds = 8

// restrVars is how many distinct activated halves (ManaRestrictAb with the
// non-spell flag) a table's restrictions can carry beside the spell half.
const restrVars = 6

// restrVarOf is the variant of mana ability a's spend restriction: 0 for a
// spell-only one, else one plus the index of its activated half, a
// restriction of Forge's "Spell.Artifact,Activated" shape being a different
// restriction from the bare "Spell.Artifact" of another source (CR 106.6).
// Read-only; a half past restrVars, or one never registered, reads as 0.
func (g *genPrep) restrVarOf(a *kop.Ability) uint8 {
	var nc uint8
	if a.ManaNoCounter {
		nc = restrVarNoCtr // AddsNoCounter$: its own batch kind (mana_nocounter.go)
	}
	ab, ns := a.ManaRestrictAb, a.AFlags&kop.AfManaNonSpell != 0
	if ab == 0 && !ns {
		return nc
	}
	for i := range g.restrVar {
		if g.restrVar[i].ab == ab && g.restrVar[i].ns == ns && (ab != 0 || ns) {
			return uint8(i) + 1 | nc
		}
	}
	return nc
}

// regRestrVar registers the activated half of mana ability a.
func (g *genPrep) regRestrVar(a *kop.Ability) {
	ab, ns := a.ManaRestrictAb, a.AFlags&kop.AfManaNonSpell != 0
	if ab == 0 && !ns {
		return
	}
	for i := range g.restrVar {
		if g.restrVar[i].ab == ab && g.restrVar[i].ns == ns && (ab != 0 || ns) {
			return
		}
		if g.restrVar[i].ab == 0 && !g.restrVar[i].ns {
			g.restrVar[i].ab, g.restrVar[i].ns = ab, ns
			return
		}
	}
}

// addRestrF registers restriction f (genPrep.restrF).
func (g *genPrep) addRestrF(f kop.FilterID) { g.addRestrFV(f, 0) }

// addRestrFV registers restriction f with activated-half variant v.
func (g *genPrep) addRestrFV(f kop.FilterID, v uint8) {
	for i := range g.restrF {
		if g.restrF[i] == f && g.restrV[i] == v {
			return
		}
		if g.restrF[i] == 0 {
			g.restrF[i], g.restrV[i] = f, v
			return
		}
	}
}

// addRestrPers marks registered restriction f persistent (genPrep.restrPers):
// the DB$ Mana that floats it carries PersistentMana$ True. kgen lowers
// CantCastNonArtifactSpells (kop.SpmNonSpell) beside persistent mana alone
// and every other restriction beside none, so no restriction is both.
func (g *genPrep) addRestrPers(f kop.FilterID) {
	if kind, ok := g.restrKind(f); ok {
		g.restrPers |= 1 << (kind - 1)
	}
}

// restrPersKind: batch kind kind is a persistent restriction's
// (genPrep.restrPers).
func (g *genPrep) restrPersKind(kind uint8) bool {
	return g.restrPers>>((kind-1)%restrKinds)&1 != 0
}

// restrKind is restriction f's batch kind (its restrF index plus one); ok
// is false for a restriction past restrKinds, which a batch cannot name.
func (g *genPrep) restrKind(f kop.FilterID) (uint8, bool) { return g.restrKindV(f, 0) }

// restrKindV is restrKind for variant v; a restriction named by its spell
// half alone (an effect's, a granted ability's) takes the first kind of that
// half when no spell-only one is registered.
func (g *genPrep) restrKindV(f kop.FilterID, v uint8) (uint8, bool) {
	for i := range g.restrF {
		if g.restrF[i] == f && g.restrV[i] == v && f != 0 {
			return uint8(i) + 1, true
		}
	}
	if v == 0 {
		for i := range g.restrF {
			if g.restrF[i] == f && f != 0 && g.restrV[i]&restrVarNoCtr == 0 {
				return uint8(i) + 1, true
			}
		}
	}
	return 0, false
}

// restrFOf is the restriction filter that batch kind kind names.
func (g *genPrep) restrFOf(kind uint8) kop.FilterID {
	return g.restrF[(kind-1)%restrKinds]
}

// restrIsKind: mana ability a is a source of the restriction batch kind
// kind names: its spell half and its activated half are the batch's.
func (g *genPrep) restrIsKind(kind uint8, a *kop.Ability) bool {
	i := (kind - 1) % restrKinds
	return a.ManaRestrict == g.restrF[i] && g.restrVarOf(a) == g.restrV[i]
}

// addRestr floats n restricted mana of colour col, spendable only as
// restriction f allows, beside p's pool (State.rb). Mana of a restriction
// past restrKinds, or past the batches p's pool holds, floats unrestricted
// in the pool instead (spilled: false).
func (s *State) addRestr(k *prep, p uint8, f kop.FilterID, col int, n uint8) bool {
	return s.addRestrCh(k, p, f, 0, col, n)
}

// addRestrCh is addRestr from a source that chose creature type ch.
func (s *State) addRestrCh(k *prep, p uint8, f kop.FilterID, ch uint8, col int, n uint8) bool {
	return s.addRestrChV(k, p, f, 0, ch, col, n)
}

// addRestrChV is addRestrCh from a source whose restriction has activated-half
// variant v (restrVarOf).
func (s *State) addRestrChV(k *prep, p uint8, f kop.FilterID, v, ch uint8, col int, n uint8) bool {
	if col >= kop.NumManaKinds {
		return false
	}
	kind, ok := k.gen.restrKindV(f, v)
	if !ok || !s.rbPut(p, kind, ch, col, n) {
		s.pl[SeatIx(p)].pool[col] += n
		return false
	}
	return true
}

// poolProj is p's pool as gorge's Players[p].Pool holds it: the
// restricted mana counted with its colour.
func (s *State) poolProj(p uint8) [kop.NumManaKinds]uint8 {
	m := s.pl[SeatIx(p)].pool
	s.rbUnits(p, s.rbAll(p), &m)
	return m
}

// restrOptions is p's plain priority walk at a node of gorge's prefix search:
// the search prices a play through the planner, which reads the floating pool
// through the restriction (pay.PlanPool), so restricted mana counts exactly
// when this play may spend it. The planner's verdict is a payment, not a new
// activation: p's this-turn Exhaust fact is read as at the search's root
// (static_turnbits.go withRootExhaust).
func (s *State) restrOptions(k *prep, d *Decision, p uint8) {
	saved := s.withRootExhaust(p)
	s.plainOptions(k, d, 0)
	s.g.turnF = saved
}

// pendListenerOrder sorts the triggers queued since lo into listener order
// (p0's battlefield order, then p1's; stable): one gorge event's
// listeners are walked once, so a combat damage event to a player queues
// its damage triggers and its life-loss triggers (Bloodthirsty Conqueror)
// interleaved by listener, not damage first (the pair 15166). A
// trigger whose source left the battlefield keeps its place at the end.
func (s *State) pendListenerOrder(lo int) {
	key := func(e *pendingTrig) uint32 {
		o := e.obj
		if o == NoObj || s.objs[o].zone != kop.ZoneBattlefield || s.objs[o].slot == NoSlot {
			return 1 << 31
		}
		p := &s.perms[s.objs[o].slot]
		return uint32(p.ctrl)<<30 | uint32(p.ts)
	}
	for i := lo + 1; i < int(s.npend); i++ {
		e := s.pend[i]
		ke := key(&e)
		j := i
		for j > lo && key(&s.pend[j-1]) > ke {
			s.pend[j] = s.pend[j-1]
			j--
		}
		s.pend[j] = e
	}
}

// fireDelayedAt fires the object delayed triggers after the end step's
// printed ones (queued since m) and, on a generated table, sorts them in
// among those by listener order: gorge's "at the beginning of the next
// end step" on a token copy (Electroduplicate's sacrifice) is a trigger
// of that token, walked at its place in the battlefield order
// (Slumbering Cerberus, the copy, the other Cerberus).
func (s *State) fireDelayedAt(k *prep, m int) {
	// The granted turn's ExtraTurnDelayedTrigger loss is registered at the
	// consumption, before any promise made during that turn: gorge queues it
	// ahead of them, so the drain (last queued resolves first) resolves
	// those, then the loss, then the printed triggers.
	m2 := int(s.npend)
	s.exRideFire()
	r := int(s.npend) - m2
	s.fireDelayed(k)
	if k.gen != nil && m2 > m && int(s.npend) > m2+r {
		s.pendListenerOrderSkip(m, m2, r)
	}
}

// pendListenerOrderSkip is pendListenerOrder over the triggers queued since
// lo with the r records at [at, at+r) (the extra-turn riders) held out of
// the sort and put back at at, after the printed triggers' share.
func (s *State) pendListenerOrderSkip(lo, at, r int) {
	if r == 0 {
		s.pendListenerOrder(lo)
		return
	}
	var held [maxExRide]pendingTrig
	copy(held[:r], s.pend[at:at+r])
	n := int(s.npend)
	copy(s.pend[at:], s.pend[at+r:n])
	s.npend = uint8(n - r)
	s.pendListenerOrder(lo)
	copy(s.pend[at+r:n], s.pend[at:n-r])
	copy(s.pend[at:], held[:r])
	s.npend = uint8(n)
}

// restrSpendable is the mask of p's restricted batches (State.rb) that may
// be spent on p's cast of card c: the card matches the batch's restriction
// as a spell (gorge AvailableFor's RestrictValidMatches: Giada's
// "Spell.Angel", Courtyard's "Spell.Creature+ChosenType", the type the
// batch's source chose). Out of line with the other cold window code.
//
//go:noinline
func (s *State) restrSpendable(k *prep, p uint8, c CardID, og restrOrigin) uint8 {
	if k.gen == nil {
		return 0
	}
	var m uint8
	for i := range s.rb[SeatIx(p)] {
		b := &s.rb[SeatIx(p)][i]
		if b.kind == 0 {
			break
		}
		if b.units() != 0 && s.restrSpendableB(k, p, b, c, og) {
			m |= 1 << i
		}
	}
	return m
}

// restrSpendableB: batch b may be spent on p's cast of card c.
func (s *State) restrSpendableB(k *prep, p uint8, b *rbatch, c CardID, og restrOrigin) bool {
	for f := k.gen.restrFOf(b.kind); f != 0; f = k.t.Filters[f].Or {
		fl := k.t.Filters[f]
		if fl.Special != 0 && !(DebugActOnlyRestrSpell && fl.Special&kop.SpmActivated != 0) {
			continue // a Static.* alternative: only a special action may spend it; the SpmActivated marker is refused here on purpose
		}
		fl.Special = 0
		fl.Zone = kop.ZoneNone // the spell is matched before it moves to the stack
		chosen := fl.Flags2&kop.F2ChosenType != 0
		fl.Flags2 &^= kop.F2ChosenType
		if !s.restrFilterMatch(k, &fl, c, SeatIx(p), og) {
			continue
		}
		if !chosen || int(b.chosen) < len(k.t.Subtypes) && k.t.Cards[c].Subtypes.Has(int(b.chosen)) {
			return true
		}
	}
	return false
}

// wideGapsOnly: p's census gaps are wide any-colour sources alone
// (restrWideAnyMana), so gorge's relaxed proof reads each one against the
// play it prices.
func (s *State) wideGapsOnly(k *prep, p uint8, restr, anyR slotMask) bool {
	if !k.gen.anyMultiR || !anyR.any() || restr.any() || s.scriptGaps(k, p).any() ||
		s.scriptCtr(k, p).any() || s.scriptFilt(k, p).any() || s.scriptAnyR(k, p).and(s.scriptWide(k, p)) != anyR || k.gen.anyComboAny && s.caGaps(k, p).any() {
		return false
	}
	return true
}

// scriptWide: p's activatable wide any-colour restricted sources.
func (s *State) scriptWide(k *prep, p uint8) slotMask {
	var m slotMask
	for b := s.scriptAnyR(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if restrWideAnyMana(&k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))]) {
			m = m.with(sl)
		}
	}
	return m
}

// wideAdmits reports whether some wide source of anyR may spend its mana on
// play a: a cast its restriction matches by the spell's card and where it is
// cast from, or an activation of a source it matches. gorge's relaxed proof
// drops the source's alternatives from every other play's payment
// (potentialRestrictionAdmits), so such a play is proven insufficient and
// takes none of the prefix searches.
func (s *State) wideAdmits(k *prep, a Action, p uint8, anyR slotMask) bool {
	if a.Kind() == AkActivate {
		ob := &s.objs[Obj(a.Source())]
		d := payDesc{class: pdAct, card: ob.card, src: NoSlot}
		if ob.zone == kop.ZoneBattlefield {
			d.src = ob.slot
		}
		for b := anyR; b.any(); b = b.rest() {
			sl := b.first()
			if s.restrSrcAdmits(k, p, sl, &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))], &d) {
				return true
			}
		}
		return false
	}
	if !isCastKind(a.Kind()) {
		return false
	}
	o := Obj(a.Source())
	c := s.objs[o].card
	og := s.restrOriginOfObj(o)
	for b := anyR; b.any(); b = b.rest() {
		sl := b.first()
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))]
		for f := ab.ManaRestrict; f != 0; f = k.t.Filters[f].Or {
			fl := k.t.Filters[f]
			if fl.Special != 0 {
				continue // a Static.* alternative: only a special action may spend it
			}
			fl.Zone = kop.ZoneNone // the spell is matched before it moves to the stack
			if s.restrFilterMatch(k, &fl, c, SeatIx(p), og) {
				return true
			}
		}
	}
	return false
}

// floatRestrFrom floats n of colour col from source sl's restriction f, recording the
// creature type the source chose when the restriction reads it: the restriction is the
// source's as it was made (CR 106.6), so mana of another source's choice floats in a
// batch of its own.
func (s *State) floatRestrFrom(k *prep, p uint8, sl Slot, f kop.FilterID, col int, n uint8) bool {
	return s.floatRestrV(k, p, sl, f, 0, col, n)
}

// floatRestrAb is floatRestrFrom for mana ability ab, whose activated half
// names the batch with its spell half.
func (s *State) floatRestrAb(k *prep, p uint8, sl Slot, ab *kop.Ability, col int, n uint8) bool {
	if ab.ManaEach {
		// an all-at-once restricted source (Tin Street Gossip's {R}{G}) floats
		// every colour it makes, not the one colour a normal tap is "for"
		ok := true
		for c := 0; c < 5; c++ {
			if ab.ManaColors&(1<<c) != 0 && !s.floatRestrV(k, p, sl, ab.ManaRestrict, k.gen.restrVarOf(ab), c, n) {
				ok = false
			}
		}
		return ok
	}
	return s.floatRestrV(k, p, sl, ab.ManaRestrict, k.gen.restrVarOf(ab), col, n)
}

func (s *State) floatRestrV(k *prep, p uint8, sl Slot, f kop.FilterID, v uint8, col int, n uint8) bool {
	return s.addRestrSrcV(k, p, sl, f, v, col, n)
}

// addRestrSrcV is addRestrSrc for restriction variant v.
func (s *State) addRestrSrcV(k *prep, p uint8, sl Slot, f kop.FilterID, v uint8, col int, n uint8) bool {
	if k.artMana && k.cards[s.cardAt(sl)].manaTyped {
		// gorge tags the unit by its producer (effects.ManaProducerTag) on
		// every route the mana takes into the pool, a pair unit's larger
		// alternative and a filter's restricted output too: a typed unit is
		// spent after the plain ones of its colour (rbTyped)
		s.g.poolArt[SeatIx(p)] |= poolArtRestr
	}
	var ch uint8
	for g := f; g != 0; g = k.t.Filters[g].Or {
		if k.t.Filters[g].Flags2&kop.F2ChosenType != 0 {
			ch = uint8(s.perms[sl].chosen)
		}
	}
	return s.addRestrChV(k, p, f, v, ch, col, n)
}

// restrAvail adds to av the restricted mana of p's that the cast of card c
// may spend (restrSpendable), reporting whether there was any.
func (s *State) restrAvail(k *prep, p uint8, c CardID, og restrOrigin, av *avail) bool {
	m := s.restrSpendable(k, p, c, og)
	if m == 0 {
		return false
	}
	s.restrAvailAdd(p, m, av)
	return true
}

// restrAvailAdd adds p's restricted mana in the batches of mask m to av.
func (s *State) restrAvailAdd(p, m uint8, av *avail) {
	var t [kop.NumManaKinds]uint8
	s.rbUnits(p, m, &t)
	for c, n := range t {
		if n != 0 {
			av.addUnits(1<<c, n)
		}
	}
}

// restrShort is cost less the restricted mana p's cast of card c may
// spend (restrSpendable): its colour pip first, then generic. A lowering
// taps the sources for this remainder only; the payment spends the
// restricted mana (payRestr).
//
//go:noinline
func (s *State) restrShort(k *prep, p uint8, c CardID, og restrOrigin, cost kop.ManaCost) kop.ManaCost {
	short, _ := s.restrShortL(k, p, c, og, cost)
	return short
}

// restrShortAdm is cost less p's restricted mana in the batches of mask m,
// known spendable on it: each colour's pips, then generic.
func (s *State) restrShortAdm(p, m uint8, cost kop.ManaCost) kop.ManaCost {
	c, _ := s.restrShortLeft(p, m, cost)
	return c
}

// restrAdmShortCost is the cast's cost less the restricted mana its payment
// spends first: a rider plan that tapped a wide restricted source floated its
// mana (State.rb), and the payment spends the batches this cast admits first
// (payRestrSpend), so the plan taps only for what the admitted restricted
// mana leaves (gorge pay.PlanPool, pay/plancost.go).
func (s *State) restrAdmShortCost(k *prep, e *stackEntry, cost kop.ManaCost) kop.ManaCost {
	if !s.rbAny(e.ctrl) {
		return cost
	}
	if m := s.restrSpendable(k, e.ctrl, e.card, s.restrOriginOfEntry(e)); m != 0 {
		return s.restrShortAdm(e.ctrl, m, cost)
	}
	return cost
}

// restrShortLeft is restrShortAdm returning the leftover L: the restricted
// units it could not fold into the cost's pips or generic. They may still pay
// any part of the cost, {X} included (CR 106.6), so an {X} witness credits
// them (lowerWitnessEx).
func (s *State) restrShortLeft(p, m uint8, cost kop.ManaCost) (kop.ManaCost, uint8) {
	var t [kop.NumManaKinds]uint8
	s.rbUnits(p, m, &t)
	var n uint8
	for c, u := range t {
		d := min(u, cost.Pips[c])
		cost.Pips[c] -= d
		n += u - d
	}
	L := n - min(n, cost.Generic)
	cost.Generic -= min(n, cost.Generic)
	return cost, L
}

// restrShortL is restrShort returning the leftover L (restrShortLeft).
func (s *State) restrShortL(k *prep, p uint8, c CardID, og restrOrigin, cost kop.ManaCost) (kop.ManaCost, uint8) {
	m := s.restrSpendable(k, p, c, og)
	if m == 0 {
		return cost, 0
	}
	return s.restrShortLeft(p, m, cost)
}

// payRestrRider is payRestr with the may-play any-colour rider.
func (s *State) payRestrRider(k *prep, p uint8, cost *kop.ManaCost, x int, o Obj, anyColor bool) {
	og := s.restrOriginOfObj(o)
	if e := s.spellEntryOf(o); e != nil {
		og = s.restrOriginOfEntry(e) // the push is behind the payment: the honest read
	}
	m := s.restrSpendable(k, p, s.objs[o].card, og)
	if m == 0 {
		s.payRider(k, p, cost, x, o, anyColor)
		return
	}
	s.payRestrSpend(k, p, m, cost, x, o, anyColor)
}

// payRestrSpend is the payment of payRestrRider once p's restricted mana
// in the batches of mask m is known spendable on it: spent before the
// ordinary mana of its colour.
func (s *State) payRestrSpend(k *prep, p, m uint8, cost *kop.ManaCost, x int, o Obj, anyColor bool) {
	pl := &s.pl[p]
	if k.gen.rs != nil && (k.gen.rs.wide || k.gen.rs.over) || s.rbColours(p, m) > 1 {
		// a widened pair or a restricted ability of several mana may leave
		// restricted units in the pool; several restricted colours
		// (mana_reflected.go)
		s.payRestrX(k, p, m, cost, x, o, anyColor)
		return
	}
	var tk [maxRB][kop.NumManaKinds]uint8
	var rn [kop.NumManaKinds]uint8
	s.rbUnits(p, m, &rn)
	before := pl.pool
	s.rbTake(p, m, &tk)
	if cost.Hyb[0] != 0 {
		h := s.hybResolveRider(k, p, *cost, x, anyColor)
		cost = &h
	}
	s.tapForRider(k, p, cost, x, o, anyColor)
	var ord [kop.NumManaKinds]uint8
	for c := range ord {
		ord[c] = before[c] + (pl.pool[c] - before[c] - rn[c]) // what the taps added joins the plain units
	}
	pre := pl.pool
	s.payFromPoolRider(k, p, cost, x, anyColor)
	s.rbSettle(p, &tk, &ord, &pre, s.rbTyped(p))
}

// Secluded Courtyard as a census gap (genPrep.anyR). gorge's planner
// census covers its {C} ability but not its restricted any-colour one, so
// with it untapped the census is incomplete and a play the planner proves
// insufficient gets a prefix search (potentialPrefixScript) that
// activates it, branching on the ability ({C} first, then "Add any
// color") and the colour (W U B R G): the first answer after which the
// play is priced (the restriction is not read there) is the script. The
// any-colour mana floats restricted (State.rb, Courtyard's restriction:
// creature spells of the chosen type), so the play completes only when
// the pool pays it (restrSpendable).

// restrAnyTapMana: a is a plain tap mana ability of a choice of colours
// with a spend restriction (Courtyard's second ability).
func restrAnyTapMana(a *kop.Ability) bool {
	return a.ManaRestrict != 0 && bits.OnesCount8(uint8(a.ManaColors&kop.ColAny)) > 1 && a.AFlags&kop.AfLatent == 0 &&
		a.Cost.Flags == kop.CostTap && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
}

// DebugWideAnyPriced makes a wide any-colour restricted source an ordinary
// restricted planner unit (two mana the planner counts; the model gorge's
// census does not follow), mutation testing only: the lockstep must catch it
// (TestWascastrestrictWebWatchMutation). Set before the table is prepared.
var DebugWideAnyPriced bool

// restrWideAnyMana: a is a restricted tap mana ability of two mana, each of
// any colour ("Add two mana in any combination of colors", Interdimensional
// Web Watch's Combo Any x2). gorge's planner cannot price it (a Combo Any
// has no colour list, PaymentPlanChoiceColours), so it is a census gap whose
// activation the prefix search branches on, a pair of colours per answer
// (scriptDFS: the multisets, W W first). Its only mana ability is the gap.
func restrWideAnyMana(a *kop.Ability) bool {
	return !DebugWideAnyPriced && restrAnyTapMana(a) && a.AFlags&kop.AfManaChoice != 0 && a.ManaN == 2 && a.ManaColors&kop.ColAny == kop.ColAny
}

// wideAnswer decodes prefix-search answer c of a wide any-colour gap: the
// c'th multiset of two colours in gorge's answer order (W W, W U ... W G,
// U U ...: the first slot never above the second; the mirrored pairs make
// the same pool, which the search has visited).
func wideAnswer(c int) (int, int) {
	for a := 0; a < 5; a++ {
		if c < 5-a {
			return a, a + c
		}
		c -= 5 - a
	}
	return 4, 4
}

const wideAnswers = 15 // the multisets of two colours of five

// wideEngineAnswer maps the script search's answer index c of a wide
// two-unit any-colour ask to the engine-order answer wideAnswer decodes
// (scriptEngineAnswer). The ask's options each carry a single ManaSymbol
// (mana_activation.go lays the two units out unit-major), so gorge's
// scriptOrderAnswers reorders its answers: a pair naming a colour the play
// announces comes first, engine order kept inside each half -- the same
// reorder mana_mixgaps.go enumerates in its own walk, mirrored here on the
// multisets the search keeps (a mirrored pair names the same colours, so
// dropping it does not change the order).
func wideEngineAnswer(need [kop.NumManaKinds]bool, c int) int {
	if c >= wideAnswers {
		return c // beyond the multisets: engine order as before
	}
	var order [wideAnswers]uint8
	no := 0
	for i := 0; i < wideAnswers; i++ {
		a, b := wideAnswer(i)
		if need[a] || need[b] {
			order[no] = uint8(i)
			no++
		}
	}
	for i := 0; i < wideAnswers; i++ {
		a, b := wideAnswer(i)
		if !need[a] && !need[b] {
			order[no] = uint8(i)
			no++
		}
	}
	return int(order[c])
}

// firstManaAb is cd's first own mana ability (-1 none).
func firstManaAb(cd *kop.CardDef) int {
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 {
			return j
		}
	}
	return -1
}

// scriptAnyR: p's activatable any-colour restricted sources (Courtyard).
func (s *State) scriptAnyR(k *prep, p uint8) slotMask {
	if !k.gen.anyAnyR {
		return slotMask{}
	}
	return s.gapMask(k, p, gfAnyR)
}

// availAnyR: in the gaps walk a Courtyard's unit ({C} as a source) is a
// unit of any type: its relaxed alternatives (paymentPlanRelaxProof).
func (s *State) availAnyR(k *prep, p uint8, a *avail) {
	for b := s.scriptAnyR(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if k.gen.anyRefl && debugReflMutate != 2 {
			// gorge's relaxed proof appends the missed ability's relaxed
			// alternatives as a unit of their own beside the source's
			// census unit (pay/plancost.go PaymentPlanRelaxed), so a Plaza
			// counts twice (mana_reflected.go)
			a.addUnits(0x3f, 1)
			continue
		}
		if cd := &k.t.Cards[s.cardAt(sl)]; restrWideAnyMana(&cd.Abilities[k.gen.anyR(s.cardAt(sl))]) {
			if s.g.availX&availNoWide == 0 {
				a.addUnits(0x3f, 2) // (no census unit of its own to replace: its only ability is the gap)
				if s.wideSecond(k, sl) {
					s.wide2RelaxExtra(k, sl, a) // (beside its census unit, mana_restrictedcombo.go)
				}
			}
			continue
		}
		pc := s.manaPCGen(k, sl)
		a.remove(s.srcMaskAt(pc, sl), max(pc.manaN, 1))
		a.addUnits(0x3f, 1)
	}
}

// scriptGapTap taps gap sl for the script search's answer c of play a: the
// search enumerates a Taps-deferred or restricted any-colour gap's colour
// answers in scriptOrderAnswers' order (the play's announced colours first,
// gorge's scriptSearch ask node), while tapGapChoice decodes engine order,
// so the answer index is translated first. The wide two-unit Courtyard ask's
// answers are colour pairs, but each of the ask's options carries a single
// ManaSymbol, so scriptOrderAnswers reorders them the same way
// (wideEngineAnswer, the mirror of mana_mixgaps.go's wide reorder). A
// Courtyard's c 0 is the "choose a mana ability" ask's printed ability (no
// ManaSymbol): engine order there. A counter-cost gap (Vivid Grove's second
// ability) its counters and mana, the ask's colour (ctrChainTap).
//
// It reports whether the tap could be made. The only failure is a life-only
// gap whose life cost a prefix's earlier activation has already spent, or
// would take the payer to 0 life (lifeGapSurvives): the shared life total is
// no part of the search's state, so a branch can outrun it. scriptDFS treats that as a failed branch and backtracks; falling
// through to the ordinary gap path would tap a source with no census ability
// (eachFee's each-colour index is -1).
func (s *State) scriptGapTap(k *prep, p uint8, a Action, sl Slot, ctr, restr, anyR, tapsDef, ca slotMask, c int, spill *[kop.NumManaKinds]uint8) bool {
	if _, life := s.lifeGapSource(k, sl); life {
		if !s.lifeGapSurvives(k, p, sl) {
			return false
		}
		if s.lifeGapAnswers(k, sl) > 1 {
			// one of any colour: the answer names a colour in the ask's
			// scriptOrderAnswers order, which puts the play's announced
			// colours first (lifeGapDo takes the engine-order colour)
			var order [5]uint8
			for i := range order {
				order[i] = uint8(i)
			}
			needFirst(order[:], s.playNeed(k, a))
			c = int(order[c])
		}
		s.lifeGapDo(k, p, sl, c) // pay the life, float the mana (mana_lifegap.go)
		return true
	}
	if ca.has(sl) {
		if s.anyDifferentAt(k, sl) {
			// an AnyDifferent source: c is a rank among its distinct-colour
			// answers (anyDifferentAnswers), mapped to the odometer answer
			n := max(int(s.anyDifferentAbilityAt(k, sl).ManaN), 1)
			need := s.playNeed(k, a)
			s.anyDifferentTap(k, p, sl, anyDifferentRank(n, &need, c))
			return true
		}
		if k.isReflAmt(s.cardAt(sl)) {
			s.reflAmtChainTap(k, p, a, sl, c) // a reflected Amount$ source's colour ask (mana_reflamt.go)
			return true
		}
		s.caChainTap(k, p, a, sl, c) // a Combo Any source's colour ask (mana_comboany.go)
		return true
	}
	if ctr.has(sl) {
		s.ctrChainTap(k, p, a, sl, ctr, restr, c, &[kop.NumManaKinds]uint8{})
		return true
	}
	if restr.has(sl) && k.gen.anyDeferred && k.deferIdent(s.cardAt(sl)) {
		s.deferChainTap(k, p, a, sl, c) // a deferred identity source's colour ask (mana_deferident.go)
		return true
	}
	if tapsDef.has(sl) && s.riderPlainFirst(k, sl) {
		s.riderPlainGap(k, p, a, sl, c) // the plain first ability and the rider's colours are one ask (mana_rider.go)
		return true
	}
	if tapsDef.has(sl) && s.restrCensusTapsGapShape(k, sl) {
		// the census float and the restricted colours are one ask
		// (mana_tapsdefer.go)
		s.restrCensusDeferGap(k, p, a, sl, c)
		return true
	}
	s.tapGapChoice(k, p, sl, restr, anyR, tapsDef, s.scriptEngineAnswer(k, a, sl, anyR, tapsDef, c), spill)
	return true
}

// scriptEngineAnswer maps the script search's answer index c of gap sl to
// the engine-order answer tapGapChoice decodes (scriptGapTap).
func (s *State) scriptEngineAnswer(k *prep, a Action, sl Slot, anyR, tapsDef slotMask, c int) int {
	if !anyR.has(sl) && !tapsDef.has(sl) {
		return c
	}
	var mask uint8
	var chosenPC *pcard
	multi := false
	off := 0
	if anyR.has(sl) {
		ab := &k.t.Cards[s.cardAt(sl)].Abilities[k.gen.anyR(s.cardAt(sl))]
		if restrWideAnyMana(ab) {
			if s.wideSecond(k, sl) {
				return wide2EngineAnswer(s.playNeed(k, a), c)
			}
			return wideEngineAnswer(s.playNeed(k, a), c)
		}
		mask = uint8(ab.ManaColors)
		if c == 0 {
			return 0
		}
		off, c = 1, c-1
	} else if a, ok := s.riderAbilityOf(k, sl); ok {
		mask = uint8(a.ManaColors)
	} else {
		pc := s.blgGapPC(k, sl) // an intrinsic-unit rider source's answers are its rider's colours (blgGapPC)
		mask = s.srcMaskAt(pc, sl)
		if pc.manaChosen {
			chosenPC = pc
		} else if pc.manaMulti {
			// a multi-ability fold (mana_multicolour.go): kgen orders the
			// abilities W U B R G C, so mask-ascending is the engine order
			// the ask's options follow. gorge's scriptOrderAnswers reads
			// only Option.ManaSymbol, and a multi-ability source's "Choose
			// a mana ability" options (engine/rules/mana_activation.go) set
			// it only on the Combo-flattened and stage-2 colour options,
			// never here, so it keeps that order: no needFirst hoist.
			multi = true
		}
	}
	var order [kop.NumManaKinds]uint8
	no := 0
	if chosenPC != nil {
		no = s.chosenAskOrder(k, sl, chosenPC, (*[5]uint8)(order[:5])) // gorge's ask: printed colours, then the chosen one
	} else {
		for col := uint8(0); col < kop.NumManaKinds; col++ {
			if mask&(1<<col) != 0 {
				order[no] = col
				no++
			}
		}
	}
	if c >= no {
		return off + c // beyond the mask's colours: engine order as before
	}
	if !multi {
		needFirst(order[:no], s.playNeed(k, a)) // scriptOrderAnswers on the ask
	}
	col := order[c]
	if tapsDef.has(sl) {
		return bits.OnesCount8(mask & ((1 << col) - 1)) // tapDeferFloat's engine-order index
	}
	return off + int(col) // floatRestrAb takes the colour itself
}

// tapGapChoice taps gap sl for p with gorge's search answer c: a Courtyard
// (anyR) its first ability ({C}) for c 0, else colour c-1 of its
// restricted one (State.rb); a Taps-deferred gap (tapsDef) its answer's
// colour into the plain pool (mana_tapsdefer.go); any other gap as tapGap
// does. spill counts restricted mana the pool's batches could not hold
// (floating unrestricted: addRestr).
func (s *State) tapGapChoice(k *prep, p uint8, sl Slot, restr, anyR, tapsDef slotMask, c int, spill *[kop.NumManaKinds]uint8) {
	cd := &k.t.Cards[s.cardAt(sl)]
	if !anyR.has(sl) && !tapsDef.has(sl) {
		var held [kop.NumManaKinds]uint8
		rb := s.rb[SeatIx(p)]
		s.tapGap(k, p, sl, restr, &held)
		if restr.has(sl) && s.rb[SeatIx(p)] == rb && !k.gen.has(s.cardAt(sl), gfDeferred) {
			a := &cd.Abilities[k.gen.restr(s.cardAt(sl))]
			spill[bits.TrailingZeros8(uint8(a.ManaColors))] += s.restrAbN(k, sl, a)
		}
		return
	}
	if tapsDef.has(sl) {
		if _, ok := s.riderAbilityOf(k, sl); ok {
			s.riderGap(k, p, sl, c)
			return
		}
		s.perms[sl].flags |= pfTapped
		s.touchSl(sl)
		s.manaTapped(k, sl)
		if k.gen != nil && k.gen.anyManaRider && s.manaPCGen(k, sl).manaIntrinsic {
			// the view is the granted types' intrinsic unit (blgView): the tap
			// is still the deferred rider's activation, the prefix search's
			// relaxed alternative -- its rider resolves with the mana
			// (manaRider's riderDeferred gate; CR 605.3b)
			s.manaRider(k, sl)
		}
		s.tapDeferFloat(k, p, sl, c)
		return
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if ab := &cd.Abilities[k.gen.anyR(s.cardAt(sl))]; restrWideAnyMana(ab) {
		if s.wideSecond(k, sl) { // its first ability's colours, then the multisets (mana_restrictedcombo.go)
			s.wide2Float(k, p, sl, c, spill)
			return
		}
		c1, c2 := wideAnswer(c)
		for _, col := range [2]int{c1, c2} {
			if !s.floatRestrAb(k, p, sl, ab, col, 1) {
				spill[col]++
			}
		}
		return
	}
	if c == 0 {
		f := &cd.Abilities[firstManaAb(cd)]
		s.pl[p].pool[bits.TrailingZeros8(uint8(f.ManaColors))] += max(f.ManaN, 1)
		return
	}
	if !s.floatRestrAb(k, p, sl, &cd.Abilities[k.gen.anyR(s.cardAt(sl))], c-1, 1) {
		spill[c-1]++
	}
}

// anyRAnswers: the answers of gap sl's activation ask in the script search (a
// Courtyard's six; a wide source's colour multisets).
func (s *State) anyRAnswers(k *prep, sl Slot) uint8 {
	if cd := &k.t.Cards[s.cardAt(sl)]; restrWideAnyMana(&cd.Abilities[k.gen.anyR(s.cardAt(sl))]) {
		if s.wideSecond(k, sl) {
			return wide2Answers
		}
		return wideAnswers
	}
	return 6
}

// scriptSkip is a path entry whose gap the script does not tap: gorge's
// search (scriptSearch.visit) branches every remaining gap as a node's
// child, so after a Taps-deferred gap's own answers failed the line goes
// on without it -- beside Manabarbs the only paying plan taps the Islands
// and skips the unneeded lands, whose taps would kill the payer before the
// play (cast of Charix, the Raging Isle, 2026-11-19). A gap whose watcher
// is kind to its payer still never needs the skip: mana only adds up.
const scriptSkip = 0xff

// scriptSubtreeCost prices a subtree of m gaps left under the tapped ones
// as scriptPostNode prices gorge's walk: a chain of up to n+1 clones per
// subset. Past 24 gaps the count is past gorge's cap many times over; the
// shift stops there so a large board cannot wrap the int into a pass.
func scriptSubtreeCost(m, n int) int {
	if m > 24 {
		m = 24
	}
	return (1 << m) * (n + 1)
}

// scriptSkipCost prices, as scriptPostNode prices gorge's walk, the clones
// gorge spends on the tap-subtrees its search exhausts before each skip of
// path: reaching a branch that went on without gap j, gorge first visits
// every subset that keeps it, a chain of up to n+1 clones each. 0 when the
// line skipped nothing. j indexes path and the order it walks: the callers
// pass the live prefix only, so a gap a foreign ask cut out of the order
// (cutAll) prices nothing.
func scriptSkipCost(path []uint8, n int) int {
	cost := 0
	for j, v := range path {
		if v == scriptSkip {
			cost += scriptSubtreeCost(n-j-1, n)
		}
	}
	return cost
}

// scriptMultiGap reports whether any gap of the search multiplies the walk's
// nodes: the counter, Combo Any, Taps-deferred and life gaps' answers.
func scriptMultiGap(ctr, ca, life, tapsDef slotMask) bool {
	return ctr.any() || ca.any() || life.any() || tapsDef.any()
}

// scriptNodeFound applies gorge's cap to a found node past the branch's skips:
// the walk's taps, the pass per pending trigger (a sorcery-speed play waits for
// the stack) and the skipped subtrees' clones must fit the cap, or the node
// offers nothing. A node with no skip keeps the caller's result.
func scriptNodeFound(found bool, tapsN, depth, npend int, path []uint8, n int, resolve bool) bool {
	if !found || tapsN == depth {
		return found
	}
	pend := 0
	if resolve {
		pend = npend
	}
	return tapsN+pend+scriptSkipCost(path[:depth], n) < potentialScriptNodes
}

// scriptCutAll records gap ans's answers: every one tried so far queued another
// player's ask. A skipped gap has no answers of its own, so it is no
// continuation of any prefix.
func scriptCutAll(cut bool, ans uint8, prev bool) bool {
	return cut && ans != scriptSkip && (ans == 0 || prev)
}

// scriptSkipGap takes the branch that goes on without gap j when j is a
// Taps-deferred gap and its skipped tap-subtree fits gorge's cap; otherwise it
// clears a stale skip left from an earlier descent. It reports whether the line
// went on without the gap (the branch's last child, as gorge's resolve is the
// node's last arm).
func scriptSkipGap(path []uint8, j, n int, defers bool) bool {
	if path[j] == scriptSkip {
		path[j] = 0 // stale skip of a gap a fresh descent taps again
		return false
	}
	if !defers {
		return false
	}
	if scriptSkipCost(path[:j], n)+scriptSubtreeCost(n-j-1, n) < potentialScriptNodes {
		path[j] = scriptSkip
		return true
	}
	return false
}

// scriptPostFound applies the same cap to a node's post pass past the branch's
// skipped subtrees: gorge reaches it only within its cap.
func scriptPostFound(post bool, tapsN, depth, n int, path []uint8) bool {
	if !post || depth == 0 || tapsN == depth {
		return post
	}
	return tapsN+scriptSkipCost(path[:depth], n)+scriptSubtreeCost(n-depth, n) < potentialScriptNodes
}

// scriptDFS is gorge's prefix search (scriptSearch.visit) for play a: p's
// gaps tapped in battlefield order (order[i:]), a Courtyard branched on its
// six answers, a counter-cost gap with a plain {T} first mana ability on
// the two branches its activation ask lists (the {T} tap, then the
// counter-cost one -- an any-colour one on its five colour answers),
// depth first; at each node after the first tap the
// walk (restricted mana priced as ordinary) is asked for a. It returns the
// number of gaps the found script reaches (path holds their answers, and
// scriptSkip where the line went on without the gap) and,
// in dead, whether the oracle cannot complete it: restricted mana floats
// and the play's pool does not pay it (the planner refuses that pool).
// Mana only adds up, so skipping a gap never prices what tapping it does
// not -- unless the tap costs the payer (a Taps-deferred watcher that
// hurts it, scriptSkip below), and the chain with its branches is the
// search's order.
// not, and the chain with its branches is the search's order.
//
// The walk is iterative over one copy of s (a recursive walk's per-node
// copy escapes to the heap): each node rebuilds the copy from s by
// replaying path's taps, so the play loop allocates nothing.
func (s *State) scriptDFS(k *prep, a Action, p uint8, order []Slot, ctr, restr, anyR, tapsDef, ca slotMask, path []uint8, dead *bool) (int, bool) {
	var d Decision
	life := s.lifeGapMask(k, p) // a life-only gap among the order (mana_lifegap.go)
	depth := 0                  // gaps tapped at this node: order[:depth], answers path[:depth]
	nodes := 0
	stall := uint8(0) // the first stall a node's fork latched (t.g.stalled), StallNone none
	tapsN := 0        // the current branch's tapped gaps: gorge's descent, one clone each
	// An activation's granted mana is the grant search's only for a play its
	// walk lists (grantCap). A play the walk leaves off -- it needs a
	// Taps-deferred or counter-cost gap (Saprazzan Skerry), which is no unit
	// of that walk -- reaches only this search, and gorge's one prefix search
	// activates the grants beside that gap (scriptSearch.visit): count them
	// as units here, as scriptDeadEnd does, unless a live grant carries a
	// spend rule availFor does not read (restrictedGrantLive). Ebony Fly's
	// {4} roll from Gemhide Sliver's and Mothdust Changeling's granted mana
	// and Skerry's {U}{U} (CR 605.1a, 601.2g-h).
	noGrant := a.Kind() == AkActivate && s.restrictedGrantLive(k, p)
	// post: the node is revisited once its children are exhausted (gorge's
	// visit runs the pass that lets the queued triggers resolve only after
	// every deeper activation failed, mana_scriptpost.go)
	post := false
	var cutAll [MaxPerm]bool // gap j: every answer tried so far queued another player's ask
	for {
		replay, cut := true, false
		if depth > 0 {
			if nodes++; scriptMultiGap(ctr, ca, life, tapsDef) && nodes > potentialScriptNodes { // gorge's search budget: no script past it (the counter, Combo Any, Taps-deferred and life gaps' answers multiply)
				if stall != 0 {
					// the drop is final (scriptDeadEndAny's callers drop the
					// play) and one node's fork ran into a rule gap the kernel
					// stalls by name: declare it, not "no payment" (tapXScript's
					// stall; a found path is not this, its replay latches on s)
					s.capFull(stall)
				}
				return 0, false
			}
			t := s.fork()
			t.g.noGrant = noGrant
			var spill [kop.NumManaKinds]uint8
			tapsN = 0 // the branch's tapped gaps: gorge's descent, one clone each
			for j := 0; j < depth; j++ {
				if path[j] == scriptSkip {
					continue // the line went on without this gap
				}
				tapsN++
				if !t.scriptGapTap(k, p, a, order[j], ctr, restr, anyR, tapsDef, ca, int(path[j]), &spill) {
					// a life-only gap the path's earlier activation made
					// unpayable: no state under this prefix can tap it, so
					// fail the branch and backtrack (see scriptGapTap)
					replay = false
					break
				}
			}
			if stall == 0 {
				stall = t.g.stalled // the chain taps may have latched a stall
			}
			t.hideDeferred(k, p, a, tapsDef)
			t.restrOptions(k, &d, p)
			// gorge's search completes at a payer priority only when the play
			// is among the decision's options (offered). A trigger the path's
			// taps queued (a Taps-deferred source the search gap-activates,
			// or an ordinary activation's own trigger) the search leaves
			// pending while it works down the gaps -- the walk keeps offering
			// the play there, gorge's engine menu does not -- and answers
			// (its order ask, the search's scriptAnswers) only when the gaps
			// are done (or the play is instant-speed and waits for nothing):
			// the copy places it, and -- the play sorcery-speed, waiting for
			// the stack (waitsForStack: the search branches on the pass that
			// lets it resolve, CR 307.1, 602.5d) -- resolves it with the
			// other players passing, and the play is offered again only if
			// the resolution left the game running. A resolution that ends it
			// (the Bow equipped to the source deals its damage to the last
			// opponent) ends every branch of the search before the play: no
			// script. The walk mirrors that, on the copy resolvePrefixStack
			// runs (mana_sacsource.go).
			resolve := t.playNeedsEmptyStack(k, a)
			foreign := replay && t.foreignPendAsks(k, p, resolve)
			found := replay && hasOption(&d, a) && (t.npend == 0 || depth == len(order) || !resolve || post) && !foreign
			// the branch's skips put tap-subtrees ahead of this node in gorge's
			// walk (scriptSkipCost prices them): past gorge's cap it offers
			// nothing (Spiked Corridor), so the node is not the search's to
			// judge -- its taps, the pass per pending trigger (sorcery-speed
			// plays wait for the stack) and the skipped subtrees must fit the cap
			found = scriptNodeFound(found, tapsN, depth, int(t.npend), path, len(order), resolve)
			// gorge's visit returns false at another player's ask and its
			// activation loop goes on to the next sibling: the line ends here
			// (no deeper gap is tapped under it) and a gap whose every answer
			// ends this way is no continuation of any prefix (cutAll, below)
			cut = foreign
			cutAll[depth-1] = scriptCutAll(cut, path[depth-1], cutAll[depth-1])
			if found && t.npend != 0 {
				t.resolvePrefixPlay(k, a, p, t.sp, resolve)
				if t.over {
					found = false
				} else if resolve {
					t.restrOptions(k, &d, p)
					found = hasOption(&d, a)
				}
			}
			if !found && stall == 0 {
				stall = t.g.stalled // the node's own options build or resolution may have latched one
			}
			if found {
				*dead = false
				if t.rbAny(SeatIx(p)) || spill != ([kop.NumManaKinds]uint8{}) {
					for c := range spill {
						t.pl[SeatIx(p)].pool[c] -= spill[c] // restricted mana the byte spilled pays nothing either
					}
					*dead = !t.scriptCompletes(k, p, a)
				}
				s.unfork()
				return depth, true
			}
			s.unfork()
		}
		if replay && !cut && depth < len(order) && !post { // the node's first child
			path[depth] = 0
			depth++
			continue
		}
		post = false
		for { // a leaf: the deepest gap with an answer left, else done
			if depth == 0 {
				if m, ok := s.scriptPartialResolve(k, a, p, order, ctr, restr, anyR, tapsDef, ca, dead); ok {
					// the fallback validated every gap of the prefix with its first
					// answer; the exhausted walk left path[] at the last ones, and the
					// callers replay the script from path
					clear(path[:m])
					return m, true // a shorter prefix, resolved, leaves the payer alive (mana_tapspartial.go)
				}
				if stall != 0 {
					// every line failed and one ran into a rule gap the kernel
					// stalls by name: declare it, not "no payment" (tapXScript's
					// stall; a found path is not this, its replay latches on s)
					s.capFull(stall)
				}
				return 0, false
			}
			j := depth - 1
			nch := uint8(1)
			switch {
			case ctr.has(order[j]):
				nch = uint8(max(k.ctrChainN(s.cardAt(order[j])), 1)) // the plain {T} first ability's answers, then the counter ability's (ctrChainTap)
			case anyR.has(order[j]):
				nch = s.anyRAnswers(k, order[j])
			case tapsDef.has(order[j]):
				nch = s.deferGapAnswers(k, order[j])
			case restr.has(order[j]) && k.gen.anyDeferred && k.deferIdent(s.cardAt(order[j])):
				nch = s.deferAnswers(k, order[j]) // the identity's colour ask (mana_deferident.go)
			case life.has(order[j]):
				nch = s.lifeGapAnswers(k, order[j])
			case ca.has(order[j]):
				nch = s.caGapAnswers(k, order[j])
			}
			if int(path[j])+1 < int(nch) { // int: the scriptSkip sentinel must not wrap into answer 0
				path[j]++
				break
			}
			if cutAll[j] {
				// every answer of gap j ends the line at a foreign ask (a
				// Taps watcher the opponent controls: Relic Bind on a
				// Walrus): gorge's search tries the other gaps as this
				// node's children, so the search goes on without j
				// (mana_foreignask.go). order is the callers' array: the
				// prefix they replay from path is the shortened one.
				copy(order[j:], order[j+1:])
				order = order[:len(order)-1]
				cutAll[j] = false
				depth = j
				post = false
				break
			}
			// gorge's search branches every remaining gap as a node's child
			// (scriptSearch.visit): after gap j's own answers failed the line
			// goes on without it. The skip is the branch's last child, after
			// the gap's answers, as gorge's resolve is the node's last arm
			// (mana_scriptpost.go); the replay (scriptPrefixAny) and the node
			// check (above) read the skipped gap out of path. Reaching it gorge
			// first exhausts the tap of order[j] -- the subsets that keep the
			// gap, priced as scriptPostNode prices a subtree -- so past its cap
			// (ten Manabarbs lands at five life) the branch is not the search's
			// to take and the line ends here.
			if scriptSkipGap(path, j, len(order), tapsDef.has(order[j])) {
				break
			}
			depth--
			post = scriptPostFound(s.scriptPostNode(k, a, depth, len(order)), tapsN, depth, len(order), path)
			if post {
				break
			}
		}
	}
}

// scriptGapOrder is the prefix search's gap order for play a: every gap in
// battlefield order, but not the play's own source when its ability's cost
// taps it. A permanent tapped for mana cannot also pay a {T} cost (CR 602.2b,
// 118.3), so a prefix that tapped the source could only take the play's own
// {T} away. Under Price of Glory every land of the non-active payer is a
// Taps-deferred gap, and Ramunap Ruins's "{2}{R}{R}, {T}, Sacrifice a Desert"
// was lost that way: the prefix tapped the Ruins before the other lands
// funded it (pair 131, decision 286), while the Ruins stays the untapped
// permanent its own {T} pays.
func (s *State) scriptGapOrder(k *prep, a Action, p uint8, gaps slotMask, order []Slot) int {
	if a.Kind() == AkActivate && a.Mode()&0x80 == 0 {
		if o := Obj(a.Source()); s.objs[o].zone == kop.ZoneBattlefield && s.objs[o].slot != NoSlot {
			if c := s.objs[o].card; int(a.Mode()) < len(k.t.Cards[c].Abilities) &&
				k.t.Cards[c].Abilities[a.Mode()].Cost.Flags&kop.CostTap != 0 {
				gaps = gaps.without(s.objs[o].slot)
			}
		}
	}
	return s.battlefieldOrder(p, gaps, order, 0)
}

// scriptDeadEndAny is scriptDeadEnd with a Courtyard, a Taps-deferred source
// or a counter-cost gap among the gaps.
func (s *State) scriptDeadEndAny(k *prep, a Action, p uint8, restr, anyR, tapsDef, ctr, ca slotMask) bool {
	tapsDef = s.deferGapsForPlay(k, a, tapsDef)
	var order [MaxPerm]Slot
	gaps := s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef).or(ctr).or(ca).or(s.lifeGapMask(k, p))
	n := s.scriptGapOrder(k, a, p, gaps, order[:])
	var path [MaxPerm]uint8
	dead := true
	if tapsDef.any() && !gaps.andNot(tapsDef).any() {
		// only Taps-deferred gaps: the subsets search (mana_tapsdefer.go
		// deferGapSearch), not the ordered prefixes
		var ans [MaxPerm]uint8
		var pick [MaxPerm]int8
		if !s.deferGapSearch(k, a, p, order[:n], tapsDef, nil, &ans, &pick, &dead) {
			return true
		}
		return dead
	}
	if _, ok := s.scriptDFS(k, a, p, order[:n], ctr, restr, anyR, tapsDef, ca, path[:], &dead); !ok {
		return true
	}
	return dead
}

// scriptPrefixAny is scriptPrefix with a Courtyard, a Taps-deferred source
// or a counter-cost gap among the gaps: the search's script, played.
func (s *State) scriptPrefixAny(k *prep, a Action, p uint8, restr, anyR, tapsDef, ctr, ca slotMask) {
	var d Decision
	s.plainOptions(k, &d, 0)
	if hasOption(&d, a) || s.lifePlannedPlay(k, p, a) {
		return
	}
	tapsDef = s.deferGapsForPlay(k, a, tapsDef)
	var order [MaxPerm]Slot
	gaps := s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef).or(ctr).or(ca).or(s.lifeGapMask(k, p))
	n := s.scriptGapOrder(k, a, p, gaps, order[:])
	var path [MaxPerm]uint8
	var dead bool
	if tapsDef.any() && !gaps.andNot(tapsDef).any() {
		// only Taps-deferred gaps: gorge's visit explores the gaps' subsets,
		// not the ordered prefixes scriptDFS walks (mana_tapsdefer.go
		// deferGapSearch) -- a later gap's unit, or an earlier gap whose
		// prefix's triggers end the payer beside it, is the sibling gorge's
		// search reaches next
		var ans [MaxPerm]uint8
		var pick [MaxPerm]int8
		if !s.deferGapSearch(k, a, p, order[:n], tapsDef, nil, &ans, &pick, &dead) {
			return
		}
		for j := 0; j < n; j++ {
			if pick[j] < 0 {
				continue
			}
			var spill [kop.NumManaKinds]uint8
			s.scriptGapTap(k, p, a, order[j], ctr, restr, anyR, tapsDef, ca, int(ans[j]), &spill)
			if s.npend != 0 {
				// gorge answers each gap's activation as its own decision, so the
				// trigger queue is drained before the next tap: two lands under
				// different Contaminated Grounds stack in tap order (CR 603.3b); a
				// tap that queued two of one controller's is gorge's order ask,
				// answered as the script does before the next tap
				s.flushPrefixTriggers(k)
				if s.npend != 0 {
					s.resolvePrefixStack(k, s.sp, false)
				}
			}
		}
		return
	}
	m, ok := s.scriptDFS(k, a, p, order[:n], ctr, restr, anyR, tapsDef, ca, path[:], &dead)
	if !ok {
		return
	}
	var spill [kop.NumManaKinds]uint8
	for i := 0; i < m; i++ {
		if path[i] == scriptSkip {
			continue // the found script went on without this gap
		}
		s.scriptGapTap(k, p, a, order[i], ctr, restr, anyR, tapsDef, ca, int(path[i]), &spill)
		if s.npend != 0 {
			// gorge answers each gap's activation as its own decision, so the
			// trigger queue is drained before the next tap: two lands under
			// different Contaminated Grounds stack in tap order (CR 603.3b); a
			// tap that queued two of one controller's is gorge's order ask,
			// answered as the script does before the next tap
			s.flushPrefixTriggers(k)
			if s.npend != 0 {
				s.resolvePrefixStack(k, s.sp, false)
			}
		}
	}
}

// availPool is p's floating pool alone as an avail.
func (s *State) availPool(p uint8, a *avail) {
	a.total, a.n = 0, 0
	for c := 0; c < kop.NumManaKinds; c++ {
		if n := s.pl[p].pool[c]; n > 0 {
			a.addUnits(1<<c, n)
		}
	}
}

// tgtDepReduce: card cd's cost is reduced when its target matches
// (Luminous Rebuke's "costs {3} less if it targets a tapped creature").
func tgtDepReduce(cd *kop.CardDef) bool {
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatReduceCost && a.Static.StFlags&kop.StTargetsFilter2 != 0 {
			return true
		}
	}
	return false
}

// suppressEOT sets the until-end-of-turn keyword grants of the permanent
// at sl aside as kwSup records (one per keyword, n = its grants): they
// apply again whenever sl has not lost its abilities (kwGen,
// FeatGrantedRep), until the cleanup ends them. A grant past the record
// capacity is dropped.
func (s *State) suppressEOT(sl Slot) {
	p := &s.perms[sl]
	if p.eotKw == 0 && s.nkwx == 0 {
		return
	}
	// "can't be blocked this turn" (Rogue's Passage, Kaito, Vampire
	// Gourmand) is an effect on the creature, not an ability it gains
	// (gorge: an Effect object's CantBlockBy static), so it stays
	const keep = kop.KwUnblockable
	var cnt [32]uint8
	for x := p.eotKw &^ keep; x != 0; x &= x - 1 {
		cnt[bits.TrailingZeros32(uint32(x))] = 1
	}
	for _, r := range p.eotKwRep {
		for x := r &^ keep; x != 0; x &= x - 1 {
			cnt[bits.TrailingZeros32(uint32(x))]++
		}
	}
	for i := 0; i < int(s.nkwx); i++ {
		if r := &s.kwx[i]; r.sl == sl && kop.KwMask(1)<<(r.bit&^kwSup)&keep == 0 {
			b := r.bit &^ kwSup
			cnt[b] = uint8(min(int(cnt[b])+int(r.n), 255))
		}
	}
	p.eotKw, p.eotKwRep = p.eotKw&keep, [2]kop.KwMask{p.eotKwRep[0] & keep, p.eotKwRep[1] & keep}
	if s.nkwx != 0 {
		s.dropKwRepX(sl)
	}
	for b, n := range cnt {
		if n != 0 {
			if int(s.nkwx) >= maxKwRepX {
				s.capFull(StallKwRepX)
				continue
			}
			s.kwx[s.nkwx] = kwRepX{sl: sl, bit: uint8(b) | kwSup, n: n}
			s.nkwx++
		}
	}
}

// supKw is the keywords of sl's suppressed until-end-of-turn grants.
func (s *State) supKw(sl Slot) kop.KwMask {
	var m kop.KwMask
	for i := 0; i < int(s.nkwx); i++ {
		if r := &s.kwx[i]; r.sl == sl && r.bit&kwSup != 0 {
			m |= kop.KwMask(1) << (r.bit &^ kwSup)
		}
	}
	return m
}

// midResAsk poses an as-enters choice queued while a program resolves (a
// permanent that entered mid-resolution with a ReplAfterEnter choice:
// Banner of Kinship returning from a sacrificed Banishing Light's exile).
// gorge asks it at once, with the resolving entry still on the stack, and
// the program continues after the answer (pair 4150); outside a
// resolution the queue is asked before the next priority (genAskOptions).
func (s *State) midResAsk(k *prep) bool {
	if k.gen == nil || !s.res.active {
		return false
	}
	for s.g.nask > 0 {
		a := &s.g.ask[0]
		if a.sl == NoSlot {
			return false // a returning Aura's host: asked before priority (genAskOptions)
		}
		if !s.occ.has(a.sl) || s.objs[s.perms[a.sl].obj].ver != a.ver {
			s.popAsk()
			continue
		}
		s.res.ask, s.res.askP = raGenEnterAsk, s.perms[a.sl].ctrl
		s.ctl = ctlResolve
		return true
	}
	return false
}

// lkiAttached is the permanent o was attached to as it left the
// battlefield (last known information, CR 608.2h): Adventuring Gear's
// landfall trigger pumps the creature it equipped though the Gear was
// destroyed in response (gorge Defined$ Equipped; pairs 15721 47715
// 85781). The departed permanent's slot keeps its record until a new one
// enters there; NoObj once it has, or when the host left too.
func (s *State) lkiAttached(o Obj) Obj {
	for sl := Slot(0); sl < MaxPerm; sl++ {
		if p := &s.perms[sl]; p.obj == o && p.ts != 0 && !s.occ.has(sl) && p.attached != NoSlot {
			if !s.occ.has(p.attached) {
				return NoObj
			}
			if h, ok := s.attLKIHost(o); ok {
				return h // the slot may hold a new object now (CR 400.7)
			}
			return s.perms[p.attached].obj
		}
	}
	if h, ok := s.attLKIHost(o); ok {
		return h
	}
	return NoObj
}

// attLKIHost is the versioned departure record's verdict for o: its host
// while that same incarnation is still on the battlefield, NoObj once it
// left (a token that took the host's slot is a new object, CR 400.7); ok
// is false when no record for o survives.
func (s *State) attLKIHost(o Obj) (Obj, bool) {
	for i := range s.g.attLKI {
		r := &s.g.attLKI[i]
		if r.obj == o && r.ver != 0 {
			h := &s.objs[r.host]
			if h.zone == kop.ZoneBattlefield && h.ver == r.ver {
				return r.host, true
			}
			return NoObj, true
		}
	}
	return NoObj, false
}

// payCastWindow pays the rest of a cast's cost through gorge's CR 601.2g
// cast window (a lowered or a free cast). That window's colour ask carries no
// owed cost, only the pips the caster announced. It reports false after
// reversing the cast when the window runs out of sources: the rest of the
// cost, a free cast's RaiseCost per target included, is unpayable (gorge
// payCast abortCast "cost no longer payable", CR 733.1; the taps and floating
// mana stay, as loweredWindowOK leaves them).
func (s *State) payCastWindow(k *prep, p uint8, cost *kop.ManaCost, anyType, anyColor bool) bool {
	ok, held := s.windowTapUseful(k, p, cost, cost, !anyType && !anyColor, anyColor, s.bld().pipN == 0, false)
	if s.winPaused() {
		for c := range held {
			s.pl[p].pool[c] += held[c]
		}
		return true // the pick is posed; the cast pays when the window ends
	}
	if !ok {
		for c := range held {
			s.pl[p].pool[c] += held[c]
		}
		s.reverseCast(k)
		return false
	}
	if s.rbAny(p) {
		// the window can float a restricted source's mana beside the pool (an
		// any-colour source whose restriction admits this cast); spend that
		// admitted batch first, as gorge's payment does (loweredWindowOK's
		// arm, CR 106.6)
		s.payRestrRider(k, p, cost, 0, s.bld().e.obj, anyColor)
	} else {
		s.payFromPoolRider(k, p, cost, 0, anyColor)
	}
	for c := range held {
		s.pl[p].pool[c] += held[c]
	}
	return true
}

// loweredPoolPays: the floating pool settles a lowered cast with no window,
// counting the restricted mana this spell may spend.
func (s *State) loweredPoolPays(k *prep, e *stackEntry, cost kop.ManaCost, anyColor bool) bool {
	if s.rbAny(e.ctrl) {
		cost = s.restrShort(k, e.ctrl, e.card, s.payDescNow(NoObj).og, cost)
	}
	return s.windowPoolPays(e.ctrl, &cost, anyColor)
}

// windowSrcMask is the colours the window can take from sl: its own mana
// ability's (colourless when it names none) joined with every granted ability's,
// as gorge's clamp reads the colours of each ability the source offers
// (paymentSourceUseful over availableManaAbilitiesForWindow).
func (s *State) windowSrcMask(k *prep, sl Slot) uint8 {
	mask := s.windowOwnMask(k, sl)
	if k.gen != nil && len(k.gen.grantPC) > 0 && s.grantMay(k, sl) {
		mask |= s.windowGrantMask(k, sl)
	}
	return mask
}

// windowOwnMask is the colours sl's own mana ability makes in the window
// (colourless when it names none).
func (s *State) windowOwnMask(k *prep, sl Slot) uint8 {
	pc := s.windowGrantPC(k, sl)
	if mask := s.srcMaskAt(pc, sl); mask != 0 {
		if k.painMana && pc.painCol != 0 && !pc.painOnly {
			// a painland: gorge's ability ask flattens the painful Combo
			// into colour options with symbols beside {C}'s symbol-less one,
			// so the window answer always takes a colour (painWindow)
			mask |= uint8(pc.painCol) & 0x1f
		}
		return mask
	}
	return 1 << kop.C
}

// windowUseful is gorge's cast-window clamp (rules/mana_activation.go
// paymentUsefulColours, read by paymentSourceUseful): once the pool's
// mana beyond the coloured pips covers the generic, only a source adding
// a pip colour the pool still lacks is offered (want, decided). A hybrid
// pip or an unsettled generic leaves it undecided: every source stays.
// An any-type rider (the caller's anyType) fails open the same way.
func (s *State) windowUseful(k *prep, p uint8, cost *kop.ManaCost) (want uint8, decided bool) {
	if cost.Hyb[0] != 0 {
		return 0, false
	}
	pool := &s.pl[p].pool
	if m := s.restrPoolAdmits(k, p); m != 0 {
		// gorge prices the pool AvailableFor holds: the restricted mana this payment may spend counts (CR 106.6)
		var t [kop.NumManaKinds]uint8
		t = *pool
		s.rbUnits(p, m, &t)
		pool = &t
	}
	surplus := 0
	for c := 0; c < kop.NumManaKinds; c++ {
		if cost.Pips[c] > pool[c] {
			want |= 1 << c
		} else {
			surplus += int(pool[c] - cost.Pips[c])
		}
	}
	if surplus < int(cost.Generic) {
		return 0, false
	}
	return want, true
}

// lowerWitness taps the witness of a cast the oracle lowers (beginCast):
// gorge's composed-cost plan of the printed mana symbols, the card being
// cast counting toward the hand demand (see beginCast's kicked case).
// When the printed symbols have no plan, a cast offered only through its
// permission's any-type rider (the default surface without
// Config.GorgeUnpriced: the oracle never offers it) prices the any-type
// cost, as tapPrinted does, and taps nothing when that has no plan
// either: the payment then pays from the window (the self-play panic
// "autopay found no plan"). The lowering waits out what the taps trigger
// before the cast (resolveLowerCounterTriggers), whichever cast route lowers.
func (s *State) lowerWitness(k *prep, p uint8) {
	n0 := s.npend
	s.lowerWitnessEx(k, p, NoObj)
	if s.buildLive() && s.bld().obsSl != 0 && s.bld().obsWin&obsWinPaying == 0 {
		s.bld().obsWin |= obsWinTapped // the witness floats its mana before the announced X prices the cast (mana_spendtrigger.go obsWinSettle)
	}
	if s.npend > n0 {
		b := s.bld()
		s.resolveLowerCounterTriggers(k, n0, b.e.kind == seSpell && (k.cards[b.e.card].instant || s.castFlash(k, b.e.ctrl, b.e.card)))
	}
}

// lowerWitnessEx is lowerWitness with the hand card excl left out of the
// witness's hand demand (a plain cast's PlanCastPayment witness).
func (s *State) lowerWitnessEx(k *prep, p uint8, excl Obj) {
	s.g.noTgtRed = true
	any := s.bld().e.flags & sfAnyMana
	anyColor := s.bld().bfl&bflAnyColor != 0
	s.bld().e.flags &^= any // the witness prices the printed mana symbols
	s.bld().bfl &^= bflAnyColor
	cost := s.buildCost(k)
	s.bld().e.flags |= any
	if anyColor {
		s.bld().bfl |= bflAnyColor
	}
	var av avail
	// The witness prices the cast census (legal.go priorityOptions): a cast's
	// census leaves a dynamic {T}, sacrifice source out (avail.dynSac stays
	// false), as does the payment planner (planCastDynSacSkip): gorge funds a
	// cast with it only by activating the source at priority, its script
	// route's prefix float (mana_sacctr.go), so the floating pool already
	// holds that mana when the cast's witness plans. Counting it here priced
	// an {X} cast at an X the plan could not tap, and the payment panicked
	// "autopay found no plan" (tail-34-c001 pair 9, Crackle with Power).
	av.dynSac = false
	if cost.X != 0 && s.bld().e.kind == seSpell {
		if held := s.sacPartReserved(k, p, spellAddCost(k, s.bld().e.card)); held.any() {
			s.ctrlM[p] = s.ctrlM[p].andNot(held)
			defer func() { s.ctrlM[p] = s.ctrlM[p].or(held) }()
		}
	}
	s.availFor(k, p, &av)
	printedPays := false
	if any != 0 || anyColor {
		// gorge prices the printed symbols on its census, which has no
		// granted source (rules potentialModeCastPlan: the rider only when
		// that plan is insufficient; mana_grantedoffers.go), so a granted
		// token's mana cannot keep the printed price (CR 609.4b)
		pv := &av
		var ug avail
		if k.gen != nil && len(k.gen.grantPC) != 0 {
			s.availUngranted(k, p, &ug)
			pv = &ug
		}
		ok := pv.affords(&cost, pipMaskOf(&cost), 0)
		if anyColor {
			printedPays = ok // gorge's witness plans the printed symbols when they are payable, the rider's generic price only otherwise
			ok = pv.affordsAnyColor(&cost, 0)
		}
		if !ok && !anyColor && k.restrSrc && s.bld().e.kind == seSpell {
			// the printed symbols are paid by a restricted source the spell
			// may spend (Lilypad Village's {U} for a creature spell): it is a
			// census source of gorge's witness, so the any-type price below
			// does not apply (CR 106.6)
			ok = s.restrSrcAffords(k, p, s.payDescNow(excl), &cost, pv, NoSlot, 0)
		}
		if !ok {
			cost = s.buildCost(k)
			if cost.Hyb[0] == 0 {
				ok = av.affords(&cost, pipMaskOf(&cost), 0)
				if anyColor {
					ok = av.affordsAnyColor(&cost, 0)
				}
			} else if cost.Hyb[0] != kop.HybTwobrid {
				// the printed hybrid symbols have no plan (an unannounced
				// hybrid cost, the may-play cast this rider offered): price
				// the rider's cost the way the offer gate did
				// (planUnderRider: every face of a hybrid pip is payable by
				// any mana, one generic each)
				cost = planUnderRider(cost, any != 0, anyColor)
				ok = av.affords(&cost, 0, 0)
				if anyColor {
					ok = av.affordsAnyColor(&cost, 0)
				}
			} else {
				ok = true
			}
			if !ok && (!s.spellTgtDep(k, s.bld().e.ctrl, s.bld().e.card, s.bld().e.obj) ||
				severalTargets(k, s.bld().e.card) && s.bld().e.flags&(sfMayPlay|sfFlashback) != 0) {
				s.g.noTgtRed = false
				return
			}
		}
	}
	s.g.noTgtRed = false
	s.witnessAtCheapest(k, p, &cost, &av)
	var L uint8
	if s.rbAny(SeatIx(p)) && s.bld().e.kind == seSpell {
		// the witness starts from the mana this payment may spend (pay.PlanPool)
		if m := s.restrSpendable(k, p, s.bld().e.card, s.restrOriginOfObj(s.bld().e.obj)); m != 0 && cost.Hyb[0] != 0 && cost.Hyb[0] != kop.HybTwobrid {
			// a hybrid pip resolves in gorge's planHybridCost order over that
			// pool (the first resolution with a witness wins), so the
			// restricted mana stands in the pool while the plan taps for the
			// resolved remainder: {R/W}{R/W} evoked with Flamebraider's two
			// floating {R} taps nothing more, and {1}{U/R}{U/R} cast with its
			// {W}{U} and a Treasure's {U} taps no further
			pl := &s.pl[SeatIx(p)]
			var tk [maxRB][kop.NumManaKinds]uint8
			s.rbTake(p, m, &tk)
			defer func() {
				// the units go back to their batches (rbTake left them in
				// place, emptied), beside the restricted mana the plan's own
				// taps floated since (witnessManaRestricted): Lilypad
				// Village's {U} beside the Mentor's {G} (CR 106.6)
				for i := range tk {
					for c, n := range tk[i] {
						pl.pool[c] -= n
						s.rb[SeatIx(p)][i].n[c] += n
					}
				}
			}()
		} else {
			cost, L = s.restrShortL(k, p, s.bld().e.card, s.restrOriginOfObj(s.bld().e.obj), cost)
		}
	}
	cost, _ = s.waterbendBuildCredit(k, p, cost)
	x := 0
	if cost.X != 0 {
		// gorge's witness for an {X} spell pays the largest X the
		// sources can (rules potentialModeCastPlan): the printed ones (a
		// granted ability is no witness source; mana_grantedoffers.go)
		if k.gen != nil && len(k.gen.grantPC) != 0 {
			s.availUngranted(k, p, &av)
		}
		if anyColor {
			// the leftover L is the restricted mana the fixed pips and
			// generic could not fold: it still pays any part of the cost, so
			// the witness prices each X with it credited (CR 106.6); with
			// L = 0 maxXAnyColorLeft is the plain maxXAnyColor
			x = max(0, av.maxXAnyColorLeft(&cost, L))
		} else {
			x = max(0, av.maxXLeft(&cost, L, pipMaskOf(&cost)))
		}
		// the search is bounded by the mana the pool and every source can
		// hold, over the X count, before any reduction (rules
		// potentialModeCastPlan: hi = paymentPlanCensusTotal / base.X): a
		// reduction paying into {X} cannot lift the witness past it; L is
		// pool mana gorge's census total counts, and the painful units
		// painTight held out of a are census sources of the total (censusTotal)
		x = min(x, (av.censusTotal()+int(L))/int(cost.X))
		// the restricted sources the payment may spend (the Emperor of
		// Palamecia's {U} or {R}) are census sources of gorge's witness
		// too: tapForRider's plan taps them, so count them and cap on the
		// same restricted-inclusive total
		if k.restrSrc {
			d := s.payDescNow(excl)
			t, multi, nm := s.restrSrcAvail(k, p, d, &av, NoSlot)
			// L is pool mana gorge's census total counts, as the
			// (censusTotal+L) bound above; withheld is the sources' own
			t.total = t.censusTotal()
			x = restrWitnessXPlus(x, t, multi[:nm], &cost, s.restrWithheldMana(k, p, d)+int(L)) // (mana_restrwithheld.go)
		}
	}
	if cost.X != 0 && s.bld().e.kind == seSpell && s.xWitnessEats(k, p, s.bld().e.card, &cost, x, excl, L) {
		s.lowerXEats(k, p, &cost, x, excl, anyColor)
		return
	}
	if L != 0 && cost.X != 0 {
		// the taps price the {X} folded at the witness's x with L generic
		// off: the payment spends L on it (payRestr, CR 106.6)
		cost = xLcredit(cost, x, L)
		x = 0
	}
	untapped := s.untappedRestrSrcs(k, p)
	tapColor := anyColor && !printedPays // the printed symbols the census pays are planned as printed (CR 609.4b lets either tap pay)
	cost = s.lowerWitnessKeepsTarget(k, p, excl, cost)
	if !k.restrSrc || !k.t.Cards[s.bld().e.card].HasAltAdd && cost.Hyb[0] == 0 && !s.anyRestrEach(k, untapped) {
		if !s.tapForSpellTaps(k, p, &cost, x, excl, tapColor) {
			s.tapForRider(k, p, &cost, x, excl, tapColor)
		}
		return
	}
	pool := s.pl[p].pool
	var pk planner
	s.tapForPK(k, p, &cost, x, excl, false, tapColor, &pk)
	s.witnessManaRestricted(k, p, &pool, untapped, &pk)
}

// restrWitnessX is the largest-X witness's X once the restricted sources the
// payment may spend are counted: t holds the plain units and the restricted
// ones that make fixed mana, multi each restricted evaluated amount (Helga's
// power: that many mana of one colour). The census total X divides into
// counts the amounts too, as gorge's witness does (rules
// potentialModeCastPlan: paymentPlanCensusTotal / base.X), so Helga's two
// beside three lands lift {X}{U} to X = 4, not the lands' 3.
func restrWitnessX(x int, t avail, multi []dynUnit, cost *kop.ManaCost) int {
	return restrWitnessXPlus(x, t, multi, cost, 0)
}

// witnessAtCheapest reprices a witness the printed composition cannot fund (rules
// potentialModeCastPlan): a target-conditional reduction at the cheapest legal
// target, and a hand or may-play cast's either-or additional cost with its part
// that has mana when no part without mana can be paid (rules potentialAltAddPlan
// prices the may-play arm beside its parts like the plain cast's: the permission
// replaces the mana cost only, CR 118.9d).
func (s *State) witnessAtCheapest(k *prep, p uint8, cost *kop.ManaCost, av *avail) {
	e := &s.bld().e
	if e.kind != seSpell {
		return
	}
	cd := &k.t.Cards[e.card]
	if cd.HasAltAdd && (e.flags&sfMayPlay != 0 || s.objs[e.obj].zone == kop.ZoneHand) && !s.altAddFreePart(k, p, e.card, e.obj) {
		*cost = s.altAddReduced(k, *cost, s.altAddManaPart(k, e.card))
		return
	}
	if s.spellTgtDep(k, p, e.card, e.obj) && cost.Hyb[0] == 0 &&
		(!severalTargets(k, e.card) || e.flags&(sfMayPlay|sfFlashback) == 0) {
		funded := av.affords(cost, pipMaskOf(cost), 0)
		if k.gen != nil && len(k.gen.grantPC) != 0 {
			// the planner's census has no granted source (Imprisoned in the Moon's {C})
			funded = s.affordsUngrantedFor(k, p, cost, NoSlot, s.payDescNow(NoObj))
		}
		if !funded {
			*cost = s.buildCost(k)
		}
	}
}

// altAddReduced is the total cost of a cast paying its either-or additional
// cost's mana part: the part joins the spell's cost before the reductions
// take their generic off the total (CR 601.2f), as gorge's offer cost composes
// it, so Uncle Iroh's {1} off Redirect Lightning's {R} plus {2} leaves {1}{R}. cost is the
// spell's own priced cost, kept when the total cannot be composed again (a cost already
// fixed, or an alternative cost in build).
func (s *State) altAddReduced(k *prep, cost, part kop.ManaCost) kop.ManaCost {
	b := s.bld()
	if b.altAdd != 0 || b.bfl&bflCostFixed != 0 || (!s.statM.any() && k.gen == nil) {
		return addCost(cost, part)
	}
	fl, bf, fg := b.e.flags, b.bfl, b.fixedGen
	b.e.flags &^= sfAnyMana
	b.bfl &^= bflAnyColor
	b.altAdd = 1
	s.g.noTgtRed = true
	c := s.buildCost(k)
	s.g.noTgtRed = false
	b.altAdd = 0
	b.e.flags, b.bfl, b.fixedGen = fl, bf, fg
	return c
}

// availForBuilt is availFor for the spell being built, with the floating restricted
// mana it may spend.
func (s *State) availForBuilt(k *prep, e *stackEntry, av *avail) {
	s.availFor(k, e.ctrl, av)
	if s.rbAny(e.ctrl) {
		s.restrAvail(k, e.ctrl, e.card, s.payDescNow(NoObj).og, av)
	}
}

// witnessManaRestricted floats a witness's restricted mana as restricted: the plan
// taps a restricted source like any unit, but what it taps floats under its
// restriction, which a reversal leaves in the pool (CR 106.6, 733.1). pool is p's
// pool before the witness, untapped the restricted sources it could tap and pk
// the plan that tapped: only a source the plan tapped as its restricted unit
// floats restricted, in the colour the plan made (Turtle Lair tapped for its
// plain {C} floats nothing), and a widened unit, which may have made its own
// colour, by the mana it added to the pool.
func (s *State) witnessManaRestricted(k *prep, p uint8, pool *[kop.NumManaKinds]uint8, untapped slotMask, pk *planner) {
	for m := untapped; m.any(); m = m.rest() {
		sl := m.first()
		c := s.cardAt(sl)
		if s.perms[sl].flags&pfTapped == 0 {
			continue
		}
		tapped, ext, tcol := false, false, 0
		abi := int8(-1)
		for i := 0; i < pk.rtapN; i++ {
			if pk.rtap[i].sl == sl {
				tapped, ext, tcol, abi = true, pk.rtap[i].ext, int(pk.rtap[i].col), pk.rtap[i].ab
			}
		}
		if !tapped {
			continue
		}
		rec := k.gen.restrRecOf(c)
		if rec == nil {
			continue
		}
		if abi < 0 {
			abi = rec.ab // a record whose unit was not tagged: the first ability
		}
		ab := &k.t.Cards[c].Abilities[abi]
		n := max(ab.ManaN, 1)
		// a widened unit's tap made the colour the plan chose: when that
		// colour is none the card's own census ability makes (Turtle Lair's
		// {C}), the mana is the restricted ability's, in that colour. When the
		// census ability makes the colour, that ability paid and no restricted
		// mana floats: another tapped source's mana in the pool is not this
		// source's (a Villainous Hideout tapped for its {C} beside a Hell's
		// Kitchen's {B}).
		pick := ext
		if !ext {
			if ma := k.cards[c].manaAb; ma >= 0 && uint8(k.t.Cards[c].Abilities[ma].ManaColors)&(1<<tcol) == 0 {
				pick = true
			}
		}
		if !pick {
			// the plan tapped the source's plain unit, not its restricted
			// one: what it added is ordinary mana, whatever colour another
			// source's unit grew in beside it (CR 106.6, 601.2g). Taking a
			// unit of that colour instead would float a Swamp's {B} as the
			// Hideout's Villain-only mana.
			continue
		}
		col := tcol
		if ab.ManaEach {
			s.witnessEachRestricted(k, p, pool, sl, ab, n)
			continue
		}
		if uint8(ab.ManaColors)&(1<<col) != 0 && s.pl[p].pool[col] >= pool[col]+n {
			s.pl[p].pool[col] -= n
			if !s.floatRestrAb(k, p, sl, ab, col, n) {
				s.capFull(StallRestrSrc)
			}
		}
	}
}

// lkiCtrlSlot is lkiCtrl for target slot i's object.
func (s *State) lkiCtrlSlot(k *prep, o Obj, i int) uint8 {
	if t := tspecE(k, &s.res.e, i); t != nil && t.Kind == kop.TgtObject {
		if f := &k.t.Filters[t.Filter]; f.Zone == kop.ZoneNone && f.Flags&kop.FSpell == 0 {
			best, ts := uint8(255), uint32(0)
			for sl := Slot(0); sl < MaxPerm; sl++ {
				if p := &s.perms[sl]; p.obj == o && p.ts != 0 && !s.occ.has(sl) && uint32(p.ts) > ts {
					best, ts = p.ctrl, uint32(p.ts)
				}
			}
			if best != 255 {
				return best
			}
		}
	}
	return s.ctrlOfObj(o)
}

// selfPTLKI is the last known P/T of ability e's source, which has left
// the battlefield since (once: ver is srcVer + 1), read from its departed
// slot's record (CR 608.2h; Alesha's "mana value less than or equal to
// Alesha's power" after she was sacrificed: pairs 12267 52089); pw, t
// (the card's printed values) otherwise.
func (s *State) selfPTLKI(k *prep, e *stackEntry, pw, t int) (int, int) {
	if ob := &s.objs[e.obj]; ob.zone != kop.ZoneBattlefield && ob.ver == e.srcVer+1 {
		if r := s.srcDepartRec(e); r != nil {
			return int(r.power), int(r.tough) // the departure snapshot (zone_lkisource.go)
		}
	}
	best := s.departedSrcSlot(e)
	if best == NoSlot {
		return pw, t
	}
	return s.ptGen(k, best)
}

// departedSrcSlot is the departed battlefield slot of ability e's source,
// which has left the battlefield once since e was created (ver is srcVer +
// 1): the record of the source as it last existed there (gorge
// bindSourceLKI's departure snapshot, CR 608.2h). NoSlot while the source is
// on the battlefield, after a second zone change, or once the slot is gone.
func (s *State) departedSrcSlot(e *stackEntry) Slot {
	ob := &s.objs[e.obj]
	if ob.zone == kop.ZoneBattlefield || ob.ver != e.srcVer+1 {
		return NoSlot
	}
	best, ts := NoSlot, uint16(0)
	for sl := Slot(0); sl < MaxPerm; sl++ {
		if p := &s.perms[sl]; p.obj == e.obj && p.ts != 0 && !s.occ.has(sl) && p.ts > ts {
			best, ts = sl, p.ts
		}
	}
	return best
}

// selfCountersLKI is the departed slot of ability e's source for a
// CardCounters read of its own source (gorge departedSourceLKI: Incendiary's
// fuse counters after it went to the graveyard with its damage trigger
// waiting, CR 608.2h); NoSlot when the read is live.
func (s *State) selfCountersLKI(e *stackEntry, o Obj) Slot {
	if e == nil || e.kind == seSpell || o == NoObj || o != e.obj {
		return NoSlot
	}
	return s.departedSrcSlot(e)
}

// digMove puts DigUntil's revealed card o into zone z. gorge moves the
// revealed cards one at a time, the ones not moved yet still in the
// library: when o is Progenitus going to a graveyard (shuffled into the
// library instead), the cards still to move (next, then after) go back on
// top in reveal order first, so the shuffle sees gorge's library, and are
// taken out of it again when their turn comes (pair 45767).
func (s *State) digMove(k *prep, p uint8, o Obj, z kop.Zone, next, after []Obj) {
	if s.objs[o].zone == kop.ZoneLibrary {
		s.pl[p].libRemove(o)
	}
	s.objs[o].zone = kop.ZoneNone
	if z == kop.ZoneGraveyard && s.g.on && s.graveRedirect(k, s.objs[o].card, z, kop.ZoneLibrary) == kop.ZoneLibrary {
		pl := &s.pl[p]
		for i := len(after) - 1; i >= 0; i-- {
			if x := after[i]; s.objs[x].zone == kop.ZoneNone {
				s.objs[x].zone = kop.ZoneLibrary
				pl.libTop(x)
			}
		}
		for i := len(next) - 1; i >= 0; i-- {
			if x := next[i]; s.objs[x].zone == kop.ZoneNone {
				s.objs[x].zone = kop.ZoneLibrary
				pl.libTop(x)
			}
		}
	}
	s.placeFrom(o, z, false, kop.ZoneLibrary, false)
}

// divPicks lists entry e's target slot's picked objects in pick order (the
// first in tgt[slot], then its extra objects), legal or not.
func (s *State) divPicks(e *stackEntry, slot uint8, codes *[maxXTgt + 1]TCode) int {
	m := 0
	if c := e.tgt[slot]; c != NoTarget {
		codes[m] = c
		m++
	}
	x := s.xtOf(e)
	for i := 0; i < int(x.n); i++ {
		if x.slot[i] == slot {
			codes[m] = x.code[i]
			m++
		}
	}
	return m
}

// startCastDivide enters bsDivide: gorge's CR 601.2d division of a divided
// slot's total among its targets, each at least 1 (rules divisionAsk).
func (s *State) startCastDivide(k *prep) {
	s.bld().stage = bsDivide
	s.bld().divLeft = 0
	e := &s.bld().e
	if k.gen == nil || !k.gen.has(e.card, gfDivCard) || e.ab >= abCascade {
		return
	}
	a := s.buildAbility(k)
	for ti := 0; ti < kop.MaxTargets; ti++ {
		t := tspecE(k, e, ti)
		if t == nil || t.Divide == 0 || t.TFlags&kop.TgfAtResolution != 0 {
			continue
		}
		if a != nil {
			if tm := a.TgtModes[ti]; tm != 0 && tm&a.ModeSet(e.mode) == 0 {
				continue
			}
		}
		var codes [maxXTgt + 1]TCode
		n := s.divPicks(e, uint8(ti), &codes)
		tot := int(t.Divide)
		if t.Divide == kop.DivLKIP1P1 {
			tot = s.divLKITotal(e) // ctr_divided.go
		} else if t.Divide == kop.DivMaxV {
			tot = s.divMaxVTotal(k, t, e) // ctr_divided.go
		} else if t.Divide == kop.DivValue {
			// a value total (Ureni's land count): gorge divisionTotalFor
			// resolves DividedAsYouChoose$ as the ability is put on the stack
			tot = s.divValueTotal(k, t, e) // tgt_variablemax.go
		} else if DebugDivNOneEach {
			// the ctr_divided.go mutation: a literal total is one per target
			tot = n
		}
		if n == 0 || tot < n {
			return
		}
		x := s.xtMut(e)
		if x == nil {
			return
		}
		for i := 0; i < n; i++ {
			x.div[i] = 1
		}
		s.bld().divSlot = uint8(ti)
		if n == 1 {
			// gorge announces the one target's whole share (CR 601.2d): the
			// total is fixed now, not re-read at resolution
			x.div[0] = uint8(tot)
			return
		}
		s.bld().divLeft = uint8(tot - n)
		return
	}
}

// castDivideOptions poses one point of the division: one DkChoose/
// ChCastDivide option per target, in pick order (gorge divisionDecision).
func (s *State) castDivideOptions(k *prep, d *Decision) {
	e := &s.bld().e
	var codes [maxXTgt + 1]TCode
	n := s.divPicks(e, s.bld().divSlot, &codes)
	d.Kind, d.Sub, d.Player = DkChoose, ChCastDivide, e.ctrl
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, codes[i], 0, 0))
	}
}

// answerCastDivide gives target c one more point.
func (s *State) answerCastDivide(k *prep, c TCode) {
	e := &s.bld().e
	var codes [maxXTgt + 1]TCode
	n := s.divPicks(e, s.bld().divSlot, &codes)
	x := s.xtMut(e)
	for i := 0; i < n; i++ {
		if codes[i] == c {
			x.div[i]++
			break
		}
	}
	s.bld().divLeft--
}

// dealAnnouncedDivision deals a cast-time division (CR 601.2d / 608.2b):
// each still-legal target takes its announced share, an illegal one's is
// lost. False when nothing was announced.
func (s *State) dealAnnouncedDivision(k *prep, e *stackEntry, slot uint8) bool {
	if e.xi == 0 {
		return false
	}
	x := s.xtOf(e)
	if x.div[0] == 0 {
		return false
	}
	var codes [maxXTgt + 1]TCode
	var amts [maxXTgt + 1]int
	m, i := 0, 0
	if e.tgt[slot] != NoTarget {
		if c := s.tgtCode(int(slot)); c != NoTarget {
			codes[m], amts[m] = c, int(x.div[0])
			m++
		}
		i = 1
	}
	for j := 0; j < int(x.n); j++ {
		if x.slot[j] != slot {
			continue
		}
		if s.xtLegal(k, e, j) {
			codes[m], amts[m] = x.code[j], int(x.div[i])
			m++
		}
		i++
	}
	if m > 0 {
		s.damageBatch(k, codes[:m], amts[:m], true) // parked: the op pauses, the answer re-executes it
	}
	return true
}

// tapForHyb taps a hybrid cast's sources through gorge's manual CR 601.2g
// window: gorge's cast planner refuses a hybrid cost (pay.PlanCostDetail
// "cost:hybrid"), so the cast is paid by activating the first useful source
// in battlefield order (pay.ManaSourceIDs) until the pool pays, not by the
// planner's witness (whose colour-count key ranks a basic before a dual
// whatever the order). The window is greedy, so it first dry-runs
// (windowDry; a State copy only past a gap or sacrificed source); when it
// would run out of sources before the cost is paid (a dual spent
// for the wrong colour ahead of the basic) the planner witness, ranked in
// battlefield order, pays instead; on a table with a basic land type grant,
// or with no witness (the faces the fallback announced when no face was
// payable), the stranded window cancels the cast (hybStrand). It reports
// false after reversing.
//
//go:noinline
func (s *State) tapForHyb(k *prep, p uint8, cost *kop.ManaCost, x int, exclude Obj, anyColor bool) bool {
	c := cost.AtX(x)
	tb := s.buildCost0(k).Hyb[0] == kop.HybTwobrid
	ok, known := s.windowDry(k, p, &c, anyColor)
	stall := uint8(0) // the probe's latched stall, StallNone none; a route the planner prices past is not the game's
	if !known {
		probe := s.fork() // a gap or sacrificed source: the rare full probe
		ok, _ = probe.windowTapRider(k, p, &c, true, anyColor)
		stall = probe.g.stalled
		s.unfork()
	}
	if ok {
		real, held := s.windowTapRider(k, p, &c, true, anyColor)
		for col := range held {
			s.pl[p].pool[col] += held[col]
		}
		if real || s.winPaused() {
			return true
		}
		// A gap or a restricted source can still leave the dry census
		// optimistic where the real walk's answers differ (a first-only
		// filter's fee is now spent as filtDo spends it, windowDryFilter ->
		// filtFeeSpend). Fall through: the planner may price what the window
		// tapped, and hybStrand below reverses the cast (CR 601.2g-h, 733.1)
		// when neither pays, rather than finishing the build short and
		// panicking in finishBuild's plan.
	}
	if known && k.gen != nil && k.gen.blgOn {
		return s.hybStrand(k, p, &c, anyColor)
	}
	if tb {
		// A planned twobrid cast is priced by the cast planner's resolved
		// faces (planTwobridCost) and pays through the planner's witness. This
		// walk pays the lowered twobrid cast (a granted/scripted source, or a
		// face-sensitive modifier the face-blind cast price cannot compose)
		// and a planned one whose census cannot fund it (tapForSpellPlain tries
		// the plan first). A flexible source the walk tapped for an earlier
		// owed colour can
		// strand a later pip only it could make (Frontier Bivouac's {U}
		// leaves a {R} pip no source makes); gorge reverses the cast and
		// keeps the tap and its mana (CR 601.2g-h, 733.1), so the kernel
		// does too. A stall the probe latched is the strand's reason when the
		// walk would have paid: declare it, not a reversed cast (the
		// strand's drop is final).
		if stall != 0 {
			s.capFull(stall)
		}
		return s.hybStrand(k, p, &c, anyColor)
	}
	var pk planner
	pk.bfl, pk.try = true, true
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
	if pk.needT > 0 && !pk.found {
		if stall != 0 {
			s.capFull(stall) // (as above: the strand's drop is final)
		}
		return s.hybStrand(k, p, &c, anyColor)
	}
	return true
}

// lowerTargetDepCostCast sends a cast whose ValidTarget$ cost static waits
// for the chosen target through the manual CR 601.2g payment window: gorge's
// planner declines it ("shape:target_dependent_cost"), so no witness prices
// it and its payment taps the first useful source in battlefield order, not
// the planner's ObjID-ordered witness. Marking it lowered keeps the plan out
// of manaStep and sends finishBuild to its lowered window arm. A cast the
// planner prices as target-paid (targetPaidPlanned, kw_spree.go) keeps its
// witness.
func (s *State) lowerTargetDepCostCast(k *prep, e *stackEntry) {
	if s.bld().bfl&bflLowered != 0 || !cardTgtDepCost(&k.t.Cards[e.card]) || s.targetPaidPlanned(k, e.ctrl, e.obj) {
		return
	}
	s.bld().bfl |= bflLowered
}

// manaStep is the build's CR 601.2g step (bsMana): before the cost objects
// are chosen (CR 601.2h, gorge costObjectAsks after manaWindowAsk), the
// planner's sources for the mana cost are tapped into the pool, so a cost
// pick sees them tapped; bsPay spends the pool. The permanents the cost
// itself taps (an ability's {T} source, a granted ability's host, the
// elected tap-X picks) are withheld from the planner, as their taps are
// paid with the rest of the cost. A cast whose rest gorge pays through its
// mana window (a free, lowered or madness cast) or with restricted mana floating
// keeps its whole payment at bsPay.
//
// announcedPipWindow reports whether an announced flexible-pip cast pays
// through tapForHyb's manual window: every hybrid cast (the planner refuses
// the shape, and a potential-play offer sets no bflLowered), and a lowered
// twobrid cast. A planned twobrid cast falls through to tapForSpellPlain.
func (s *State) announcedPipWindow(k *prep) bool {
	return s.bld().bfl&bflLowered != 0 || !s.buildIsTb(k)
}

//go:noinline
func (s *State) manaStep(k *prep) bool {
	e := &s.bld().e
	// A plan cut stays cut for the rest of the build: the plan stopped at a
	// decision (a Treasure's sacrifice behind a replacement ask), so every
	// later re-entry pays through the manual window too. Clearing the bit
	// after its first read let a re-entry after the window's own parked
	// sacrifice plan again, and a fresh plan names an any-colour source's
	// colour by its own rule (G first) where gorge's window asks the
	// lowest, so one Treasure floated a different colour on each engine.
	planCut := s.bld().bfl&bflPlanCut != 0
	switch e.kind {
	case seSpell:
		cost := s.buildCost(k)
		if s.bld().bfl&bflLowered != 0 && s.bld().pipN != 0 {
			s.loweredPipWindow(k, e, &cost)
		}
		s.lowerTargetDepCostCast(k, e)
		if e.flags&sfFree != 0 && cost != (kop.ManaCost{}) || s.bld().bfl&bflLowered != 0 || e.trigPl == trigPlMadness && debugMadnessMutate == 0 || e.trigPl == trigPlMiracle {
			return s.xSacWindowFirst(k, e, &cost)
		}
		var saved [len(build{}.picks)]uint8
		s.holdCostTaps(k, &saved)
		wbCast := false
		if k.gen != nil && k.gen.anyWaterbend {
			_, wbCast = s.waterbendPartFor(k, e) // a waterbend cast (act_waterbend.go): its cost before the picks, read before the cost is fixed
		}
		wbFull := cost
		if wbCast && s.bld().bfl&bflCostFixed == 0 {
			wbFull = xFolded(s.waterbendFullCost(k), int(e.x))
		}
		if s.bld().manaNP == 0 && s.bld().bfl&bflPushed != 0 {
			s.bld().manaNP = 1 + uint8(s.npend) // the payment's mana abilities queue from here (unpushSpell keeps them)
		}
		s.bld().bfl |= bflManaStep | bflCostFixed
		s.convRouteFlag(k, e, cost) // the any-colour conversion's pool route rides bflAnyColor (mana_convertcreature.go)
		anyColor := s.bld().bfl&bflAnyColor != 0
		if (e.flags&sfAnyMana != 0 || anyColor) && s.bld().bfl&bflMsDone == 0 {
			s.tapRiderPlan(k, *e) // (not again after a tap-another pick: the window runs on)
		}
		if handled, stranded := s.improviseRoute(k, e, &cost, anyColor, &saved); stranded {
			return true
		} else if handled { // the plan paid: on to the epilogue below
		} else if k.gen != nil && (k.gen.anyConvoke && s.tapPaysEntry(k, e) || s.delvePaysEntry(k, e)) {
			// the announced creatures pay their share before the plan runs
			// (CR 702.51a; gorge convokeAsk before manaWindowAsk) and are
			// withheld from the planner as mana sources. Gorge's planner
			// refuses a convoke cast (payment_plan.go "shape:contribution"),
			// so the payment runs through the manual CR 601.2g window: the
			// first untapped source, repeat until the pool pays.
			hidden := s.convokeHidden()
			s.occ = s.occ.andNot(hidden)
			raw := xFolded(cost, int(e.x))
			cost = s.convokeApplyPicks(k, raw)
			if s.bld().bfl&bflMsDone == 0 && s.paymentStranded(k, e, &cost) {
				s.unhide(hidden)
				s.releaseCostTaps(k, &saved)
				s.reverseCast(k)
				return true
			}
			s.windowTapUseful(k, e.ctrl, &cost, &raw, true, anyColor, s.bld().pipN == 0, false)
			s.unhide(hidden)
		} else if wbCast && cost.X == 0 {
			s.waterbendCastWindow(k, e, &cost, &wbFull, anyColor) // act_waterbend.go
		} else if planCut {
			s.windowTapRider(k, e.ctrl, &cost, true, anyColor)
		} else if k.t.Cards[e.card].HasModeCost {
			// Spree: gorge's planner declines the face ("shape:modal_cost"),
			// so the cast pays through the manual CR 601.2g window, the
			// first useful source until the pool pays (kw_spree.go). The
			// window's colour ask has no ManaPayment: its owed colours are
			// the announced hybrid/Phyrexian faces only, so with none the
			// any-colour source answers its lowest colour (windowCol's
			// noOwed), not the pip the composed Spree cost still owes.
			s.windowTapUseful(k, e.ctrl, &cost, &cost, true, anyColor, s.bld().pipN == 0, false)
		} else if cost.X != 0 && s.cfg&cfgGorgeSurface != 0 {
			if !s.xWindowPay(k, e, &cost) {
				s.releaseCostTaps(k, &saved)
				s.reverseCast(k)
				return true
			}
		} else if k.phyMana && s.phyBuild(k) {
			// a Phyrexian cast: the plan for the cost its pips settled, armed
			// on the mana half; the manual window when none exists
			// (mana_phyrexian.go)
			half := s.buildCost0(k)
			half.Tb, half.Hyb = 0, [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
			// the restricted mana a granted source's prefix tap floated pays
			// first, as the plain cast's payment spends it (CR 106.6). Only
			// the plan arm takes the shortened cost: phyPay's manual window
			// discounts the admitted floating batches itself (windowSpellSettled,
			// phyWindowReach's restrAvailAdd), so handed the shortened cost it
			// would count that mana twice and tap short a payment the window
			// funds (as the seAbility branch's restrShortAdm note above).
			hs, cs := s.restrAdmShortCost(k, e, half), s.restrAdmShortCost(k, e, cost)
			if s.phyCastPlanSettles(k, e, hs, cs) {
				cost = cs
				s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
			} else if filtOnly := s.phyFilterStranded(k, e.ctrl, &cost); !s.phyPay(k, e.ctrl, &cost, int(e.x), e.obj) && filtOnly && !s.winPaused() {
				s.releaseCostTaps(k, &saved) // the window ended short of a cost only a filter reached
				s.reverseCast(k)
				return true
			}
		} else if s.bld().pipN != 0 && s.announcedPipWindow(k) {
			// a hybrid pip was announced: gorge's cast planner refuses a
			// hybrid cost (pay.PlanCostDetail "cost:hybrid"), so it is paid
			// through its manual CR 601.2g window, which activates sources in
			// battlefield order (pay.ManaSourceIDs), not the planner's
			// ObjID-ordered witness (pay.CompareSteps). That holds for a
			// lowered cast and for a hybrid cast offered as a potential play
			// (which sets no bflLowered). A PLANNED flexible-pip cast
			// (twobrid, whose pips the planner resolves) falls through to
			// tapForSpellPlain: its announced faces are folded into the cost,
			// so the planner ranks the resolved coloured/generic requirement
			// exactly as gorge's plan does; a witness it cannot find pays
			// through the same manual window there.
			if !s.tapForHyb(k, e.ctrl, &cost, int(e.x), e.obj, anyColor) {
				return true // stranded: the cast was reversed (static_landtype.go)
			}
		} else if s.bld().altAbort {
			// The chosen either-or part cannot be paid, so gorge pays what
			// the part owes through its fallback CR 601.2g window and then
			// reverses the cast (CR 733.1), keeping the window's taps and
			// floating mana. The window has no ManaPayment (a fallback), so
			// a colour ask takes the source's lowest colour (noOwed). It
			// runs here, at the mana stage, so a tap-another source's pick
			// is posed instead of walked over.
			c := xFolded(cost, int(e.x))
			// The plan was armed for the first payable part (planCastAltAdd): the free
			// one when it can be paid, else the mana part. A chosen part other than the
			// armed one changes the cost (the {2} chosen beside a payable behold, or the
			// behold chosen when only the {2} was payable), so the plan falls back to a
			// window with no ManaPayment (CR 601.2g).
			noOwed := s.bld().pipN == 0 && (s.bld().altAdd == 1) == s.altAddFreePart(k, e.ctrl, e.card, e.obj)
			ok, _ := s.windowTapUseful(k, e.ctrl, &c, &c, true, anyColor, noOwed, false)
			if !s.winPaused() {
				if !ok || s.bld().altAdd == 0 {
					// the window ran out, or paid the spell's own cost
					// while the chosen non-mana part stays unpayable:
					// gorge reverses the cast
					s.releaseCostTaps(k, &saved)
					s.reverseCast(k)
					return true
				}
				s.bld().altAbort = false // the window paid the chosen mana part
			}
		} else if cd := k.t.Cards[e.card]; cd.HasAltAdd && s.bld().altAdd == 1 && cd.AltAddCost.Mana != (kop.ManaCost{}) {
			// An either-or additional cost's mana part (CR 601.2b): the offer
			// priced the composed total when no mana-free part was payable
			// (plannedCastCostOf, as gorge's planCastAltAdd plans it), and the
			// choice folds that same part's mana into the cost before any
			// other cost stage (altAddAsk). Plan it here so its sources are
			// the planner's ObjID-ordered witness (pay.CompareSteps), as
			// gorge's plan is, not the manual CR 601.2g window's battlefield
			// order (pay.ManaSourceIDs). When the offer priced a mana-free
			// part instead, the seat's choice of the mana part changes the
			// cost the plan was armed for, so fall back to the window as
			// gorge's plan does. A cost static can make the two equal (a
			// reduction taking the mana part's {1}), and then the plan stands.
			if cd.BeholdN == 0 || s.altAddFreePart(k, e.ctrl, e.card, e.obj) && s.altAddPlanCostDiffers(k, cost) {
				c := xFolded(cost, int(e.x))
				// a behold carrier's fallback window has no ManaPayment: a source answers
				// its lowest colour, not the owed pip (windowCol)
				ok, _ := s.windowTapUseful(k, e.ctrl, &c, &c, true, anyColor, cd.BeholdN != 0 && s.bld().pipN == 0, false)
				if !ok && !s.winPaused() {
					// the fallback window ran out of useful sources (a colour
					// answered with no owed pip strands the payment): gorge
					// reverses the cast, keeping the window's taps and floating
					// mana (CR 733.1, as the altAbort window above)
					s.releaseCostTaps(k, &saved)
					s.reverseCast(k)
					return true
				}
			} else {
				cost = s.restrAdmShortCost(k, e, cost)
				s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
			}
		} else {
			// the unshortened cost: tapForSpellPlain's planner arms take what
			// the admitted restricted mana leaves themselves (restrAdmShortCost,
			// pay.PlanPool), and its window fallback must settle through
			// windowSpellSettled, which discounts the admitted floating batches
			// itself — handed the shortened cost it would count that mana twice
			// (as the seAbility branch's restrShortAdm note below)
			if s.tapForSpellPlain(k, e, &cost, anyColor, &saved) {
				return true
			}
		}
		s.bld().bfl &^= bflManaStep
		s.releaseCostTaps(k, &saved)
		if !s.winPaused() {
			s.bld().bfl &^= bflMsDone
		}
	case seAbility:
		cost := s.buildCost(k)
		// paymentStranded's dry walk discounts the admitted floating batches
		// itself (windowSpellSettled), so it takes the cost before
		// restrShortAdm: handed the shortened cost it would count that mana
		// twice and clear a payment the planner's plain pool cannot fund.
		strandCost := s.waterbendXCharge(k, e, cost)
		if m := s.restrActNow(k, e.ctrl); m != 0 {
			cost = s.restrShortAdm(e.ctrl, m, cost) // the sources tap for what the restricted mana leaves (pay.PlanPool)
		}
		a := &k.t.Cards[e.card].Abilities[e.ab]
		var saved [len(build{}.picks)]uint8
		s.holdCostTaps(k, &saved)
		sl := s.srcSlot()
		selfUntapped := sl != NoSlot && a.Cost.Flags&kop.CostTap != 0 && s.perms[sl].flags&pfTapped == 0
		if selfUntapped {
			s.perms[sl].flags |= pfTapped
		}
		var hostSl Slot = NoSlot
		if h := e.trigObj; a.Cost.Flags&kop.CostTapHost != 0 && h != NoObj && s.objs[h].zone == kop.ZoneBattlefield {
			if hs := s.objs[h].slot; s.perms[hs].flags&pfTapped == 0 {
				hostSl = hs
				s.perms[hs].flags |= pfTapped
			}
		}
		s.bld().bfl |= bflManaStep
		switch {
		case planCut:
			s.windowTapRider(k, e.ctrl, &cost, true, false)
		case s.paymentStranded(k, e, &strandCost):
			s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
			return true
		case cost.X != 0 && s.cfg&cfgGorgeSurface != 0:
			cc := s.waterbendXCharge(k, e, s.buildCostCharge(k))
			if !s.xWindowPay(k, e, &cc) {
				s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
				return true
			}
		case s.waterbendManualPay(k, e, &saved):
			if !s.waterbendAbilityPay(k, e, &cost, &saved, hostSl, sl, selfUntapped) {
				return true
			}
		case k.gen != nil && k.gen.anyPowerUp && a.Cost.Flags&kop.CostPowerUp != 0 && s.powerUpPlanRejected(k, a):
			if !s.powerUpAbilityMana(k, e, a, sl, &saved, hostSl, sl, selfUntapped) {
				return true
			}
		case k.phyMana && s.phyBuild(k):
			if !s.phyAbilityMana(k, e, &cost, &saved) { // the plan for the cost its pips settled; the manual window when none (mana_phyrexian.go)
				s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
				return true
			}
		case s.bld().pipN != 0 && s.hybAbilityUnwitnessed(k, e, a, sl) && !s.poolPays(e.ctrl, &cost):
			if !s.hybWindowPay(k, e, &cost, &saved, hostSl, sl, selfUntapped) {
				return true // gorge's manual window for the hybrid-only cost (mana_resolutionpay.go)
			}
		case s.bld().pipN != 0 && s.hybAbilityOffPlan(k, e.ctrl, &cost) &&
			!s.abilityPlanSettles(k, e, s.hybResolve(k, e.ctrl, s.abPlannedCost(k, a, sl, e.ctrl), 0), s.hybResolve(k, e.ctrl, s.abPlannedCost(k, a, sl, e.ctrl), 0), cost, &saved):
			// gorge has no plan for the cost the pips settled: its manual window. A
			// dry window (a source's first ability is not the colour owed, or the
			// sources run out) reverses the activation and keeps the taps, as for
			// power-up (CR 601.2g, 733.1); a paused window waits on its pick.
			if !s.windowTapHeldOK(k, e.ctrl, &cost) && !s.winPaused() {
				s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
				return true
			}
		case a.Cost.Flags&kop.CostSac != 0:
			s.tapForSacCost(k, e.ctrl, &cost, int(e.x), a, sl)
		case a.Cost.Flags&kop.CostSacSelf != 0 && k.sacMana && sl != NoSlot && s.sacSelfMana(k, sl):
			s.tapForSacSelf(k, e.ctrl, &cost, int(e.x), sl) // filter_combatwords.go
		case a.Cost.Flags&kop.CostTapX != 0 && s.committedPicks():
			s.tapForTapKept(k, e.ctrl, &cost, int(e.x), a, sl, &saved) // cost_tapkept.go
		case s.sacXWindowPays(k, e, &cost):
			// a sacrifice-another source (Bolg's Company) the planner cannot
			// price: gorge's planner declines the activation, so it pays
			// through the manual CR 601.2g window, which lists the source in
			// its battlefield place (mana_windowtapx.go). A pick it pauses on
			// re-enters here with the walk's mana already floating.
			if s.gapWindowReverse(k, e, &cost, &saved, hostSl, sl, selfUntapped) {
				return true
			}
		case s.equipGapWindowPays(k, e, &cost):
			// a target-gated equip the printed sources cannot pay: the gaps
			// (Selesnya Signet's fee) fund it through the manual CR 601.2g window
			if s.gapWindowReverse(k, e, &cost, &saved, hostSl, sl, selfUntapped) {
				return true
			}
		default:
			return s.abilityPlainMana(k, e, cost, &saved, hostSl, sl, selfUntapped)
		}
		s.bld().bfl &^= bflManaStep
		if hostSl != NoSlot {
			s.perms[hostSl].flags &^= pfTapped
		}
		if selfUntapped {
			s.perms[sl].flags &^= pfTapped
		}
		s.releaseCostTaps(k, &saved)
	}
	return false
}

// gapWindowReverse runs the manual CR 601.2g window an activation the planner
// cannot price pays through (sacXWindowPays, equipGapWindowPays above): a
// dry, unpaused window reverses the activation, keeping its taps, and the
// caller returns early; a paused window waits on its pick and the step's
// epilogue runs.
func (s *State) gapWindowReverse(k *prep, e *stackEntry, cost *kop.ManaCost, saved *[len(build{}.picks)]uint8, hostSl, sl Slot, selfUntapped bool) bool {
	ok := s.windowTapHeldOK(k, e.ctrl, cost)
	if s.winPaused() {
		return false
	}
	s.bld().bfl &^= bflMsDone
	if !ok {
		s.reverseActivation(k, saved, hostSl, sl, selfUntapped)
		return true
	}
	return false
}

// abilityPlainMana is manaStep's plain-activation arm: the planner first, so an
// ordinary cost keeps its witness, then the manual CR 601.2g window when the
// census has none. The X ask can price an activation through that window
// (availXWindow and a first-only filter's fee, filtMaxX; CR 605.3b), whose
// sources the planner does not price. It runs the step's epilogue on every
// path that keeps the activation, and reports true when the activation was
// reversed because neither reaches the cost (CR 733.1).
func (s *State) abilityPlainMana(k *prep, e *stackEntry, cost kop.ManaCost, saved *[len(build{}.picks)]uint8, hostSl, sl Slot, selfUntapped bool) bool {
	if !s.tapForTry(k, e.ctrl, &cost, int(e.x), NoObj, false) {
		c := xFolded(cost, int(e.x))
		if s.gapWindowReverse(k, e, &c, saved, hostSl, sl, selfUntapped) {
			return true
		}
	}
	s.bld().bfl &^= bflManaStep
	if hostSl != NoSlot {
		s.perms[hostSl].flags &^= pfTapped
	}
	if selfUntapped {
		s.perms[sl].flags &^= pfTapped
	}
	s.releaseCostTaps(k, saved)
	return false
}

// holdCostTaps marks the untapped tap-X and waterbend picks tapped for the
// planner (saved[i] = 1 for each it marked); releaseCostTaps undoes it.
func (s *State) holdCostTaps(k *prep, saved *[len(build{}.picks)]uint8) {
	for i := 0; i < int(s.bld().pickN); i++ {
		if s.bld().pickKs[i] != pkTapX && s.bld().pickKs[i] != pkWaterbend {
			continue
		}
		if ob := &s.objs[s.bld().picks[i]]; ob.zone == kop.ZoneBattlefield && s.perms[ob.slot].flags&pfTapped == 0 {
			s.perms[ob.slot].flags |= pfTapped
			saved[i] = 1
		}
	}
}

func (s *State) releaseCostTaps(k *prep, saved *[len(build{}.picks)]uint8) {
	for i := 0; i < int(s.bld().pickN); i++ {
		if saved[i] != 0 {
			s.perms[s.objs[s.bld().picks[i]].slot].flags &^= pfTapped
		}
	}
}

// retakeCostTaps undoes releaseCostTaps.
func (s *State) retakeCostTaps(saved *[len(build{}.picks)]uint8) {
	for i := 0; i < int(s.bld().pickN); i++ {
		if saved[i] != 0 {
			s.perms[s.objs[s.bld().picks[i]].slot].flags |= pfTapped
		}
	}
}

// reverseActivation reverses an activation mid-payment (CR 733.1). The cost's own
// taps are not paid on that abort: the picks held for the planner and the {T}
// source untap with it (gorge pays them after the mana, cast_commit.go
// EmitConvokeTaps).
func (s *State) reverseActivation(k *prep, saved *[len(build{}.picks)]uint8, hostSl, sl Slot, selfUntapped bool) {
	s.releaseCostTaps(k, saved)
	if hostSl != NoSlot {
		s.perms[hostSl].flags &^= pfTapped
	}
	if selfUntapped {
		s.perms[sl].flags &^= pfTapped
	}
	s.reverseCast(k)
}

// paymentStranded reports whether the helpers or tap-cost picks committed to the
// build leave a mana payment no way through the window can finish. A permanent so
// committed can't also tap for mana, so that answer is an illegal action, undone
// before any mana is activated (CR 601.2g-h, 602.2b, 733.1; gorge
// reverseStrandedPayment). The picks are already out of the sources; a census that
// can't price every source leaves the window open. A restricted source the
// payment may spend (Eclipsed Realms' chosen-type mana for a Treefolk) is a unit
// gorge's census prices (castWindowReachable's typed pool), so the dry walk's
// restricted stop is settled by the real walk on a fork, as targetWindowShort
// does: the window must not tap and float mana for a cast the census already
// reverses before any mana ability (CR 733.1).
func (s *State) paymentStranded(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	return s.paymentStrandedPlan(k, e, cost, false)
}

// paymentStrandedPlan is paymentStranded for a payment the planner has already
// failed to plan (planFailed): a dry walk that stops at a sacrifice-only source
// (an Eldrazi Spawn, which the planner prices as a last resort and gorge's
// census counts, castWindowPays) then leaves the payment stranded, not open.
func (s *State) paymentStrandedPlan(k *prep, e *stackEntry, cost *kop.ManaCost, planFailed bool) bool {
	if k.gen == nil || s.bld().pickN == 0 || !s.committedPicks() || !s.windowCensusComplete(k, e) {
		return false
	}
	anyColor := s.bld().bfl&bflAnyColor != 0
	ok, known, restr, sacStop := s.windowDryWalk(k, e.ctrl, cost, anyColor)
	dead := known && !ok || planFailed && sacStop
	if !known && restr {
		saved := s.fork()
		defer s.unfork()
		c := *cost
		ok, _ = s.windowTapRider(k, e.ctrl, &c, false, anyColor)
		if !ok && s.g.stalled == 0 {
			// The window's answer takes the source's first listed ability, so a
			// restricted unit whose colour a fixed pip still owes is one the
			// plain walk above cannot take: it answers the plain {C}
			// (Castle Doom's) and the pip stays unpaid. The clamp arm takes it
			// (windowRestrOwedTap: the ability is owed when the plain one's
			// colour is not), and the settlement reads the pool the payment
			// may spend, so a unit the spell admits settles (CR 106.6,
			// 601.2g-h; gorge's census prices the restricted unit in its typed
			// pool, castWindowReachable). The walk is a probe: rerun it from
			// the saved state with the clamp on, whose skips only list sources
			// a still-owed pip colour — a settled pool ends the loop first.
			s.CopyFrom(saved)
			c = *cost
			ok, _ = s.windowTapRider(k, e.ctrl, &c, true, anyColor)
		}
		stall := s.g.stalled // (read before CopyFrom puts the saved state back)
		s.CopyFrom(saved)
		if !ok && stall != 0 {
			// the walk's taps and sacrifices latched a rule-gap stall: the
			// real walk on s would latch the same at the payment — declare it,
			// not a "stranded" reversal the kernel's rules cannot back
			// (tapXScript's stall)
			s.capFull(stall)
		}
		return !ok
	}
	if dead && k.gen.anyFilt && s.scriptFilt(k, e.ctrl).any() {
		// The greedy walk never activates a paid-fee filter ({1},{T}: any
		// colour); gorge's census (castWindowReachable) searches them, so the
		// window opens when some filter set pays the cost (CR 601.2g-h).
		// availFor and scriptFilt read ctrlM, which still holds the committed
		// picks s.occ withholds: a pick with a mana ability would count a second
		// time, after convokeApplyPicks already paid its share (gorge's census
		// skips convokeCommitted). Take them out of the seat's mask meanwhile.
		c := *cost
		var av avail
		held := s.ctrlM[e.ctrl]
		s.ctrlM[e.ctrl] = held.and(s.occ)
		s.availForBuilt(k, e, &av)
		reach := s.faceReachable(k, e.ctrl, &av, &c, 0)
		s.ctrlM[e.ctrl] = held
		return !reach
	}
	return dead
}

// committedPicks reports whether a pick commits a permanent to a tap cost or as a
// convoke-family helper. A delve pick does not: it names a graveyard card, not a
// permanent, so it withholds no mana source and gorge's strand check (guarded on
// pc.Announced || pc.WindowChecked || Convoke/Taps empty, cast_payment.go) skips
// a plain delve cast -- its window opens, and a window the pool still cannot
// finish aborts only after the activations (cast_commit.go abortCast).
func (s *State) committedPicks() bool {
	for i := 0; i < int(s.bld().pickN); i++ {
		if pk := s.bld().pickKs[i]; pk == pkTapX || pk == pkWaterbend || isConvokePick(pk) {
			return true
		}
	}
	return false
}

// abilityPlanSettles reports whether gorge's auto-pay plan pays the activation's
// settled cost. A plan is armed as the activation begins, for the offer's price
// (offer, or fallback when the sources cannot fund offer), with the picks still
// untapped; a shape the planner declines, or a pool that pays, arms none. Once the
// announcements settle the cost, gorge plans again without the permanents committed
// to it (CR 601.2g-h, 602.2b: one permanent does not pay two costs;
// rearmAbilityAutoPay, announcedAutoFillPlan). The picks saved names are tapped on
// entry and on return. The plans hold printed sources only: an ability's planner
// has no prefix search, so a creature a Honest Work gives {T}: Add {C} is no unit
// of it and the manual window taps it instead (planComposedCost).
//
//go:noinline
func (s *State) abilityPlanSettles(k *prep, e *stackEntry, offer, fallback, settled kop.ManaCost, saved *[len(build{}.picks)]uint8) bool {
	if offer.X != 0 {
		return false
	}
	s.releaseCostTaps(k, saved)
	chk := planner{dry: true, noGrant: true}
	s.tapForPK(k, e.ctrl, &offer, 0, NoObj, false, false, &chk)
	armed := chk.needT != 0
	if armed && !chk.found {
		chk = planner{dry: true, noGrant: true}
		s.tapForPK(k, e.ctrl, &fallback, 0, NoObj, false, false, &chk)
		armed = chk.found
	}
	s.retakeCostTaps(saved)
	if !armed {
		return false
	}
	chk = planner{dry: true, noGrant: true}
	s.tapForPK(k, e.ctrl, &settled, int(e.x), NoObj, false, false, &chk)
	return chk.found || chk.needT == 0
}

// payAbilityCosts pays an activated ability's non-mana costs.
func (s *State) payAbilityCosts(k *prep, e *stackEntry) {
	a := &k.t.Cards[e.card].Abilities[e.ab]
	sl := s.srcSlot()
	f := a.Cost.Flags
	if f&kop.CostBlight != 0 && f&kop.CostSacSelf == 0 {
		s.payBlightPicks(k) // gorge emits the blight counters before the tap
	}
	if sl != NoSlot {
		s.touchSl(sl)
	}
	if f&kop.CostTap != 0 {
		was := s.perms[sl].flags & pfTapped
		s.perms[sl].flags |= pfTapped
		if was == 0 {
			if k.gen != nil && s.subs[kop.TrigTaps].any() {
				s.fireTaps(k, sl)
			}
			s.markTapped(k, sl) // only a tap that happens marks
		}
	}
	if f&kop.CostUntapSelf != 0 && sl != NoSlot {
		s.untapCost(k, sl)
	}
	if f&kop.CostOncePerTurn != 0 && e.ab < 8 {
		s.perms[sl].actUsed |= 1 << e.ab
	}
	if f&kop.CostLoyalty != 0 {
		s.noteLoyaltyActivation(k, sl)
	}
	if f&kop.CostLoyaltyX != 0 && sl != NoSlot {
		s.payCounterX(k, sl, int(a.Cost.Ctr), uint8(e.x)) // [-X] loyalty or SubCounter<X/KIND>
	}
	if a.Cost.Exert && sl != NoSlot {
		// Exert<1/NICKNAME> as an activation cost (CR 702.100a): the
		// one-shot no-untap mark, consumed at the controller's next untap
		// step (turn.go's pfSkipUntap)
		s.touchSl(sl)
		s.perms[sl].flags |= pfSkipUntap
	}
	if f&kop.CostOncePerGame != 0 {
		s.onceUse(k, sl, a)
	}
	if f&kop.CostRemoveCtr != 0 {
		s.perms[sl].seeCtr(s)
		s.subCtrFire(k, sl, int(a.Cost.Ctr), int(a.Cost.N)) // trig_counterremoved.go
	}
	if f&kop.CostAddCtr != 0 {
		// a counter put as a cost is no effect (CR 609.1; Doubling Season's
		// rulings): an EffectOnly$ doubler never doubles it, whatever the
		// stack holds (gorge; Forge CostPutCounter effect=false,
		// XMage PayLoyaltyCost); every other AddCounter replacement does,
		// the payer putting it (replCostCtr, repl_addcounter.go)
		n := s.placeCounters(k, sl, int(a.Cost.Ctr), int(a.Cost.N), e.ctrl, ctrPutCost)
		if n > 0 && k.anyCtrAdded {
			s.ctrNoteBy(k, sl, int(a.Cost.Ctr), e.ctrl) // a cost: gorge pay_engine publishes the payer
		}
		if n > 0 && s.subs[kop.TrigCounterAddedOnce].any() {
			// "whenever you put one or more counters on" fires for the cost's
			// placement too: the [+1] of a loyalty ability (CR 606.4, 122.6)
			s.g.lastCK = kop.CounterKind(a.Cost.Ctr)
			s.fireObj(k, kop.TrigCounterAddedOnce, sl, s.perms[sl].obj, s.perms[sl].ctrl, int16(n))
		}
	}
	if f&kop.CostPayLife != 0 {
		s.payLifeCost(k, e.ctrl, &a.Cost, uint8(e.x))
	}
	if f&kop.CostPayEnergy != 0 {
		s.payEnergy(e.ctrl, &a.Cost)
	}
	if a.Cost.Mill != 0 {
		// Cost.Mill (CR 701.17b): the payer mills the count from their own
		// library's top, where gorge's cast_commit pays it -- after the life
		// and energy parts, before the draws (cost_millcost.go). The offer
		// gates priced the library, so the count below is a guard only.
		s.millCostPay(k, e.ctrl, int(a.Cost.Mill))
	}
	if a.Cost.RevealSelf {
		// Cost.RevealSelf (the forecast family's Reveal<1/CARDNAME>, CR
		// 702.54a): the cost's only candidate is the source itself (CR 201.5;
		// cast_revealpick.go), so nothing moves: gorge settles it unasked
		// (cast_asks.go revealCostAsk) and announces it as a Note only
		// (cast_commit.go's EmitChoiceCosts). The offer gates priced the
		// source's zone, so this arm is a guard only.
	}
	if f&kop.CostTapHost != 0 {
		s.tapHostGen(k, e)
	}
	if f&kop.CostDiscardSelf != 0 {
		s.discardSelfCost(k, e)
	}
	s.payPicks(k)
	s.payPutLibPicks(k) // cost_putlibself.go
	if a.Cost.DiscardHand {
		// the whole hand leaves before the source's own sacrifice, as
		// gorge's payCast pays the discard then the sacrifice
		s.payDiscardHandCost(k, e.ctrl)
	}
	if a.Cost.Host == kop.CostHostSac {
		s.sacHostCost(k, e)
	}
	if a.Cost.Host == kop.CostHostExile {
		s.exileHostCost(k, e) // cost_exilebf.go
	}
	if a.Evidence != 0 {
		s.payCostEvidence(k) // cost_collectevidence.go
	}
	if f&kop.CostReturnSelf != 0 && sl != NoSlot {
		if k.gen != nil && s.deferSacGen(k, sl) {
			return // paySacsGen, after the mana (returnSelfDeferred)
		}
		s.leave(k, sl, kop.ZoneHand, false)
		return
	}
	if f&kop.CostExileSelf != 0 && sl != NoSlot {
		s.exileSelfCost(k, sl)
		return
	}
	if f&kop.CostBlight != 0 && f&kop.CostSacSelf != 0 {
		s.payBlightPicks(k) // before the source is sacrificed
	}
	if f&kop.CostSacSelf != 0 && sl != NoSlot {
		if k.gen != nil {
			s.bld().e.li = s.snapLKI(s.perms[sl].obj) // last known information (RefSacrificed)
			if s.deferSacGen(k, sl) {
				return // paySacsGen, after the mana
			}
		}
		s.fireSac(k, sl)
		if !s.parkCost(k, s.perms[sl].obj, parkLeave, 0) {
			s.leave(k, sl, kop.ZoneGraveyard, false)
		}
	}
}

// A tap-another-permanent mana source (Springleaf Drum, Saruli Caretaker) in gorge's
// manual cast window (CR 601.2g). The window lists every source in battlefield order and
// its answer takes the first that helps, so the Drum is activated where it stands and
// gorge asks which permanent to tap (a "tapcost" choose, only with two or more
// candidates), as the unless and ward windows do (mana_windowtapx.go). Those windows
// serve a resolution; this one serves the spell being built, in manaStep (bsMana) or
// finishBuild (bsPay): the walk stops at the source, the build waits on the pick
// (bflWinTap, ChWindowTap), and the answer taps, floats the mana and lets the stage run
// again from its start, the pool already holding what the walk took. Everything is behind
// prep.tapMana, an activation's mana-step window too; a probe on a fork still takes the first candidate.

// DebugNoCastWinTap leaves a tap-another source out of the cast window (the mutation test).
var DebugNoCastWinTap bool

// castTapX: the window being walked pays the spell the build is putting on the stack, so it
// lists a tap-another-permanent source.
func (s *State) castTapX(k *prep) bool {
	if !k.tapMana || DebugNoCastWinTap || !s.buildLive() {
		return false
	}
	b := s.bld()
	if b.e.kind == seAbility {
		// an activation's manual window (bsMana, its mana step) lists them in
		// battlefield order too (Springleaf Drum paying Figure of Fable's pip;
		// beside a sacrifice-another source, sacXWindowPays)
		return b.stage == bsMana && b.bfl&bflManaStep != 0
	}
	return b.e.kind == seSpell && (b.stage == bsMana || b.stage == bsPay || b.stage == bsAltAdd)
}

// sacXWindowPays: the activation being built pays through the manual window
// because a live sacrifice-another source (Bolg's Company) is the planner's
// census gap and the planner alone cannot pay cost (gorge armAbilityAutoPay
// declines, manaWindowAsk poses the window). A pick the window paused on
// (bflMsDone, which only the window's pause sets on an activation) keeps the
// build on the window.
func (s *State) sacXWindowPays(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	if s.bld().bfl&bflMsDone != 0 {
		return true
	}
	if !k.tapMana || DebugNoSacXMana || cost.X != 0 {
		return false
	}
	if !s.sacXLive(k, e.ctrl) {
		return s.openCensusUnpayable(k, e, cost) // the window runs dry and reverses (mana_openunpayable.go)
	}
	chk := planner{dry: true, noGrant: true}
	s.tapForPK(k, e.ctrl, cost, int(e.x), NoObj, false, false, &chk)
	return chk.needT != 0 && !chk.found
}

// isFork: s is a probe copy (fork), whose window never waits on an answer.
func (s *State) isFork() bool {
	p := s.pool
	if p == nil {
		return false
	}
	for i := 0; i < p.depth; i++ {
		if p.forks[i] == s {
			return true
		}
	}
	return false
}

// winPaused: the cast window is waiting on its tap-another pick.
func (s *State) winPaused() bool {
	return s.buildLive() && s.bld().bfl&bflWinTap != 0
}

// castWinTapPause poses the pick of the cast window's tap-another source sl, whose mana is
// colour col; false on a probe, which taps the first candidate instead.
func (s *State) castWinTapPause(sl Slot, col uint8) bool {
	if s.isFork() {
		return false
	}
	s.g.winSrc, s.g.winCol = sl, col
	if s.bld().stage == bsPay {
		s.bld().bfl |= bflWinTap | bflPayDone
	} else {
		s.bld().bfl |= bflWinTap | bflMsDone
	}
	return true
}

// castWinTapOptions builds the pick: the source's candidates in battlefield order.
func (s *State) castWinTapOptions(k *prep, d *Decision) bool {
	p := s.bld().e.ctrl
	d.Kind, d.Sub, d.Player = DkChoose, ChWindowTap, p
	if k.sacXOf(s.cardAt(s.g.winSrc)) {
		d.Sub = ChCostSacrifice // a sacrifice-another source asks which to sacrifice (gorge's "sacrifice" choose)
	}
	s.winTapOptions(k, p, d)
	return true
}

// applyCastWinTap taps the permanent picked and the source and floats the mana; the build's
// stage then runs the window on.
func (s *State) applyCastWinTap(k *prep, a Action) {
	more, cand, cand2 := s.winTapPick(k, a)
	if more {
		return // the second permanent is asked next
	}
	b := s.bld()
	b.bfl &^= bflWinTap
	s.tapXDo(k, b.e.ctrl, tapXStep{sl: s.g.winSrc, cand: cand, cand2: cand2, col: s.g.winCol})
}

// floatRestrOnly floats n of colour col from source sl's restricted ability ab
// (a larger alternative of a pair unit).
func (s *State) floatRestrOnly(k *prep, p uint8, sl Slot, ab *kop.Ability, col int, n uint8) bool {
	return s.addRestrSrcV(k, p, sl, ab.ManaRestrict, k.gen.restrVarOf(ab), col, n)
}

// altAddPlanCostDiffers: the mana part chosen beside a payable mana-free part
// composes a cost other than the one the plan was armed for (the free part's,
// planCastAltAdd). Gorge falls back to the window only then (paymentPlanCheckUnits:
// plan.Cost != the actual cost), so a reduction that makes them equal keeps the plan.
func (s *State) altAddPlanCostDiffers(k *prep, cost kop.ManaCost) bool {
	b := s.bld()
	fl, bf, fg, saved := b.e.flags, b.bfl, b.fixedGen, b.altAdd
	b.e.flags &^= sfAnyMana
	b.bfl &^= bflAnyColor | bflCostFixed
	b.altAdd = 0
	armed := s.buildCost(k)
	b.altAdd = saved
	b.e.flags, b.bfl, b.fixedGen = fl, bf, fg
	return armed != cost
}
