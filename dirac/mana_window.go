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
// tap (gorge's hasUntappedManaSource).
func (s *State) windowUnpricedOpenIf(k *prep, e *stackEntry, open bool) bool {
	if !open {
		return false
	}
	tapX := s.windowTapXLive(k, e.ctrl)
	for m := s.ctrlM[e.ctrl]; m.any(); m = m.rest() {
		if sl := m.first(); s.canTapForMana(k, sl) || s.windowGap(k, e.ctrl, sl) || tapX.has(sl) || s.restrManaUntapped(k, sl) {
			return true
		}
	}
	return false
}

// restrManaUntapped: sl is an untapped permanent with a restricted mana ability.
// gorge's untapped-source gate (untappedManaSource) reads every available mana
// ability, its restriction unread, so Interdimensional Web Watch's exile-cast
// mana keeps a pool-short candidate on the menu once the census is incomplete.
func (s *State) restrManaUntapped(k *prep, sl Slot) bool {
	pm := &s.perms[sl]
	if pm.flags&pfTapped != 0 || s.activationSick(k, sl) || s.genLost(k, sl) || DebugNoRestrManaOpen {
		return false
	}
	cd := &k.t.Cards[s.cardAt(sl)]
	for j := range cd.Abilities {
		if a := &cd.Abilities[j]; a.Kind == kop.AbMana && (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) {
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
	return s.windowTapWalk(k, p, cost, useful, clamp, anyColor, noOwed, potential, false)
}

// windowTapWalk is windowTapUseful; takeDeferred is an announced X's window
// (gorge's announcedManaWindowAsk), whose answer (payexec.WindowAnswer) is the
// auto-fill option while gorge's planner prices what is owed, else the first
// source listed. The planner never taps a deferred source (planDeferred), so
// the walk skips one while the rest still cover the cost (windowPlanCovers),
// and once they do not takes it in its battlefield place like any other.
func (s *State) windowTapWalk(k *prep, p uint8, cost, useful *kop.ManaCost, clamp, anyColor, noOwed, potential, takeDeferred bool) (ok bool, held [kop.NumManaKinds]uint8) {
	for !s.windowSpellSettled(k, p, cost, anyColor) {
		if potential {
			var pot avail
			other := payDesc{class: pdOther}
			s.availPotentialFor(k, p, &pot, &other)
			if !pot.affords(cost, pipMaskOf(cost), 0) {
				return false, held
			}
		}
		var order [MaxPerm]Slot
		n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), order[:], 0)
		sl, gap := s.windowPick(k, p, order[:n], useful, clamp, anyColor, takeDeferred && s.deferLive(k) && !s.windowPlanCovers(k, p, cost, anyColor))
		if sl == NoSlot {
			return false, held
		}
		if gap {
			if k.ctrMana && s.scriptCtr(k, p).has(sl) {
				s.ctrGap(k, p, sl) // the counter, the use, the mana (mana_countercost.go)
				continue
			}
			if (s.g.winMode != winNone || s.castTapX(k)) && s.scriptTapX(k, p).has(sl) {
				if s.windowTapXGo(k, p, cost, useful, sl, clamp, anyColor, noOwed) {
					return s.g.winMode == winNone, held // the pick is posed (a cast pauses, not fails); the answer runs the walk on
				}
				continue
			}
			if k.gen.anyFiltFirst && k.gen.has(s.cardAt(sl), gfFilt) && s.filtFirstOnly(k, sl) {
				if noOwed && clamp {
					s.windowFiltFirstFallback(k, p, sl, useful) // mana_filterfirst.go
				} else {
					s.windowFiltFirst(k, p, sl, useful, noOwed && !clamp) // mana_filterfirst.go
				}
				continue
			}
			if k.gen.anyMultiR && s.scriptWide(k, p).has(sl) {
				s.windowWideTap(k, p, sl, cost, useful, clamp, noOwed, &held)
				continue
			}
			if s.windowGapAnyColour(k, p, sl, cost, useful, clamp, noOwed) {
				continue
			}
			s.tapWindowGap(k, p, sl, &held)
			continue
		}
		if clamp && !anyColor && k.gen != nil && k.gen.anyRestrSrc && s.windowRestrOwedTap(k, p, sl, useful, noOwed) {
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
		var owed kop.ManaCost
		col := s.windowColOwed(k, p, cost, s.windowOwedCost(k, p, cost, &owed), sl, pc, anyColor, noOwed)
		if noOwed && clamp && !anyColor {
			if pc.manaMulti {
				// a cast's ability ask lists what the cost owes first (gorge owedAbilitiesFirst)
				col = s.windowCol(k, p, useful, sl, pc, false, false)
			} else if want, decided := s.windowUseful(k, p, useful); decided {
				// the colour ask lists only what the clamp keeps (gorge
				// paymentUsefulColours): with the generic covered, an
				// any-colour ability answers the owed pip
				if c, ok := s.painAnyOwed(p, useful, pc); ok {
					col = c
				} else if m := s.windowSrcMask(k, sl) & want; m != 0 {
					col = bits.TrailingZeros8(m)
				}
			}
		}
		sacOnly := windowSacOnly(k, pc) // printed or granted (mana_grantcost.go)
		if k.gen != nil && k.gen.anyRestrSrc && s.windowRestrDyn(k, p, cost, sl) {
			continue
		}
		if !sacOnly {
			s.perms[sl].flags |= pfTapped
			s.touchSl(sl)
			s.manaTapped(k, sl)
		}
		life := s.windowGrantLife(k, sl, pc, col)
		s.poolAdd(p, col, int(pc.manaN))
		s.addExtraCol(k, p, sl, pc, col)
		s.artTag(k, p, sl, col)
		if life > 0 {
			s.loseLife(k, p&1, int(life)) // a granted ability's life cost, not damage (mana_grantcost.go)
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
			// a token sacrificed to pay a cast ceases at once, as gorge pays (a
			// Treasure: payCast's tail reaches gorge's priority-boundary check,
			// engine.go checkStateBased); one sacrificed to pay mid-resolution or
			// by a planned activation's mana step (bflManaStep) waits for the
			// state-based action at the next priority (CR 704.3, 704.5d)
			if s.g.on && s.objs[o].of&ofToken != 0 && s.objs[o].zone == kop.ZoneGraveyard && s.buildLive() && (s.ctl != ctlBuild || s.bld().bfl&bflManaStep == 0) {
				s.ceaseManaToken(o)
			}
		}
	}
	return true, held
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
	}
	return s.restrPoolNonSpell(k, p)
}

// windowPlanCovers reports whether p's pool and planner census (availFor, which
// leaves a deferred source out) cover cost: gorge's window then offers its
// auto-fill plan (announcedAutoFillPlan), which the answer takes.
func (s *State) windowPlanCovers(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) bool {
	var a avail
	s.availFor(k, p, &a)
	if anyColor {
		return a.affordsAnyColor(cost, 0)
	}
	return a.affords(cost, pipMaskOf(cost), 0)
}

// windowPick is the window's next source among order (p's battlefield, in
// order): the first usable one, or a window gap (gap true); NoSlot when
// none is left. A planner-deferred source is skipped unless takeDeferred
// (windowTapWalk).
func (s *State) windowPick(k *prep, p uint8, order []Slot, useful *kop.ManaCost, clamp, anyColor, takeDeferred bool) (sl Slot, gap bool) {
	want, decided := uint8(0), false
	if clamp && !anyColor {
		want, decided = s.windowUseful(k, p, useful)
	}
	for _, o := range order {
		if s.buildLive() && s.bld().pickN != 0 && s.waterbendPicked(s.perms[o].obj) {
			continue // a waterbend pick cannot also pay the mana (gorge convokeCommitted)
		}
		if s.windowCanTap(k, o) || s.windowSacNow(k, o) {
			if !takeDeferred && s.planDeferred(k, p, o) {
				continue // the window's answer follows the planner's tier: gorge's bot never picks a deferred source (mana_tapsdefer.go)
			}
			if decided && (s.windowSrcMask(k, o)|s.windowEachMask(k, o))&want == 0 && !s.windowFiltUseful(k, p, o, want) && !s.windowRestrUseful(k, o, want) {
				continue
			}
			return o, false
		}
		if (s.g.winMode != winNone || s.castTapX(k)) && s.windowTapXOffer(k, p, o, decided, want) {
			return o, true // a tap-another source (mana_windowtapx.go)
		}
		if s.windowCtrOffer(k, p, o, decided, want) {
			return o, true // a counter-cost source (mana_countercost.go)
		}
		if k.gen != nil && k.gen.anyRestrSrc {
			if m := s.windowRestrDynMask(k, p, o); m != 0 && (!decided || m&want != 0) {
				return o, false
			}
		}
		if k.gen != nil && k.gen.anyWindowGap && s.windowGap(k, p, o) {
			c := s.cardAt(o)
			if decided && k.gen.flags[c]&gfRestr != 0 && uint8(k.t.Cards[c].Abilities[k.gen.restr(c)].ManaColors)&want == 0 {
				continue // the clamp reads a fixed colour; a Tender's has none and stays
			}
			return o, true
		}
		if k.gen != nil && k.gen.anyFiltFirst && s.windowFiltFirstUsable(k, p, o) && (!decided || s.windowFiltUseful(k, p, o, want)) {
			return o, true
		}
		if k.gen != nil && k.gen.anyMultiR && (!decided || want != 0) && s.windowWide(k, p, o) {
			return o, true // a wide any-colour restricted source (windowWideTap)
		}
	}
	return NoSlot, false
}

// windowCanTap is canTapForMana for the window's walk: a source whose only
// usable mana ability is one effects granted it, and whose grant the payment
// being built cannot spend, is not offered (CR 106.6), the same gate windowGap
// puts on a printed restricted source. A source with an admitted grant, or a
// usable printed ability, is.
func (s *State) windowCanTap(k *prep, sl Slot) bool {
	if k.gen == nil {
		return s.canTapForMana(k, sl)
	}
	return s.canTapForManaPC(k, sl, s.windowGrantPC(k, sl))
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
	if len(k.gen.grantPC) == 0 || !s.grantMay(k, sl) || s.ownManaUsable(k, sl) {
		return s.manaPCGen(k, sl)
	}
	var gi [maxGrants]int16
	n := s.grantIdxs(k, sl, &gi)
	if n == 0 {
		return s.manaPCGen(k, sl)
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

// windowColOwed is windowCol for a cost whose shortfall the pool is measured against as owed:
// cost less the restricted mana the payment may spend (windowOwedCost). cost is the full
// (raw) cost: the helpers that price the restricted pool themselves (windowDemand,
// windowUseful) read it, so the mana is not taken off it twice, while owed is the shortfall.
func (s *State) windowColOwed(k *prep, p uint8, cost, owed *kop.ManaCost, sl Slot, pc *pcard, anyColor, noOwed bool) int {
	mask := s.windowOwnMask(k, sl)
	col := bits.TrailingZeros8(mask)
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
		if prim := uint8(pc.manaCol) & 0x1f; prim != 0 {
			// A two-colour normal ability beside the painful one (Haunted
			// Screen: {W} or {B}, then {G}, {U} or {R} for 1 life): the ask
			// lists both abilities' colours, so an owed colour either
			// ability makes has answered above (owedColoursFirst) and
			// otherwise the answer is the lowest listed colour, mask's (W),
			// not the painful ability's lowest for its own sake
			// (payexec.WindowAnswer, TestPayWindowTapsLowestColour). A
			// noOwed ask (a ward/unless window) owes nothing at all, so it
			// takes the same lowest colour and pays no life. A {C} normal
			// ability (a painland, Starting Town) keeps the measured answer
			// below.
			return bits.TrailingZeros8(mask)
		}
		// a painland: gorge's "Choose a mana ability" ask carries no
		// payment, so the answer is its lowest colour option whatever
		// the window owes (mana_pain.go)
		col = bits.TrailingZeros8(uint8(pc.painCol))
		if pc.painCol == kop.ColAny {
			// Starting Town: the any-colour ability lists no symbol. A fixed
			// pip still owed makes the ability ask take it (owedAbilitiesFirst);
			// the colour ask then answers the owed pip when the demand is
			// decided, and otherwise offers every colour, where the payment
			// window's bot takes the first listed one (W), not an owed pip.
			demand := s.windowDemand(k, p, cost)
			col = kop.C
			if m := uint8(pc.painCol) & 0x1f & demand; m != 0 {
				col = bits.TrailingZeros8(uint8(pc.painCol))
				if _, decided := s.windowUseful(k, p, cost); decided {
					col = bits.TrailingZeros8(m)
				}
			}
		}
	}
	return col
}

// windowDry runs windowTapRider's source walk without its effects: only the
// pool and the tapped bits move, and both are restored before it returns.
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
	pool := s.pl[p].pool
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
		sl, gap := s.windowPick(k, p, order[:n], cost, true, anyColor, false)
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
		if gap || windowSacOnly(k, pc) || s.perms[sl].flags&pfTapped != 0 ||
			k.gen != nil && k.gen.anyRestrSrc && s.windowRestrDynMask(k, p, sl) != 0 {
			ok, known = false, false
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
	return ok, known, restr
}

// windowDryFilter is windowDry's step over a first-only filter the floating mana
// pays for (gorge's castWindowReachable, CR 605.3a): the fee comes from what cost
// can spare and the filter makes a colour cost owes, so it never loses a pip. With
// nothing to spare the swap gains nothing, and the filter only counts as spent.
func (s *State) windowDryFilter(k *prep, p uint8, cost *kop.ManaCost, sl Slot) {
	if s.filtNoTap(k, sl) {
		s.perms[sl].actUsed |= 1 << uint(k.gen.filt(s.cardAt(sl)))
	} else {
		s.perms[sl].flags |= pfTapped
	}
	pl := &s.pl[p]
	fee := s.filtFee(k, sl)
	spare := 0
	for c := range pl.pool {
		spare += int(pl.pool[c]) - min(int(pl.pool[c]), int(cost.Pips[c]))
	}
	if spare < fee {
		return
	}
	for c := range pl.pool {
		for fee > 0 && pl.pool[c] > cost.Pips[c] {
			pl.pool[c]--
			fee--
		}
	}
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
	if k.gen.flags[c]&(gfEachCol|gfRestr|gfDeferred) == 0 {
		return false
	}
	pm := &s.perms[sl]
	if pm.flags&pfTapped != 0 || s.activationSick(k, sl) ||
		s.genLost(k, sl) || pm.ovr.flags&(ovLoseAll|ovLoseAllEOT) != 0 {
		return false
	}
	if k.gen.flags[c]&(gfEachCol|gfRestr) == gfRestr {
		// CR 106.6: a restricted source whose mana the payment cannot spend is
		// not offered (gorge's window lists only the abilities the payment meets).
		d := s.payDescNow(NoObj)
		return s.restrSrcAdmits(k, p, sl, &k.t.Cards[c].Abilities[k.gen.restr(c)], &d)
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
	if !s.floatRestrAb(k, p, sl, ab, col, max(ab.ManaN, 1)) {
		s.capFull(StallRestrSrc)
	}
	return true
}

// windowAffords: gorge's mana window pays cost from p's sources counting
// the Tenders' mana (availFor does not), simulated on a copy.
func (s *State) windowAffords(k *prep, p uint8, cost *kop.ManaCost) bool {
	return s.windowAffordsTake(k, p, cost, false)
}

// windowAffordsTake is windowAffords; takeDeferred is an unless / ward window
// (payWindow), which taps a planner-deferred source (a counter-rider mana
// source, Twitching Doll) once the rest cannot cover the cost: gorge's
// unlessWindowUnits lists it, so the pay is offered (CR 118.3, 605.3b).
func (s *State) windowAffordsTake(k *prep, p uint8, cost *kop.ManaCost, takeDeferred bool) bool {
	if k.gen == nil || !(k.gen.anyEachCol && s.scriptGaps(k, p).any() || s.scriptDeferred(k, p).any() || takeDeferred && s.scriptTapsDefer(k, p).any()) {
		return false
	}
	t := s.fork()
	defer s.unfork()
	ok, _ := t.windowTapWalk(k, p, cost, cost, false, false, false, false, takeDeferred)
	return ok
}

// windowAffordsN is windowAffords for an unless cost of amt with pips.
func (s *State) windowAffordsN(k *prep, p uint8, amt int, pips int16) bool {
	c := manaOf(amt, pips)
	return s.windowAffordsTake(k, p, &c, true)
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
// with a Tender's mana (windowAffords), out of line.
//
//go:noinline
func (s *State) windowPays(k *prep, p uint8, kind int, skip uint16, amt int, pips int16) bool {
	return kind == kop.PayMana && skip&(pfWard|pfModes) != 0 && s.windowAffordsN(k, p, amt, pips)
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
	ab, ns := a.ManaRestrictAb, a.AFlags&kop.AfManaNonSpell != 0
	if ab == 0 && !ns {
		return 0
	}
	for i := range g.restrVar {
		if g.restrVar[i].ab == ab && g.restrVar[i].ns == ns && (ab != 0 || ns) {
			return uint8(i) + 1
		}
	}
	return 0
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
			if g.restrF[i] == f && f != 0 {
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
// when this play may spend it.
func (s *State) restrOptions(k *prep, d *Decision, p uint8) {
	s.plainOptions(k, d, 0)
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
	m2 := int(s.npend)
	s.fireDelayed(k)
	if k.gen != nil && m2 > m && int(s.npend) > m2 {
		s.pendListenerOrder(m)
	}
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
		if !s.restrFilterMatch(k, &fl, c, p&1, og) {
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
		s.scriptCtr(k, p).any() || s.scriptFilt(k, p).any() || s.scriptAnyR(k, p).and(s.scriptWide(k, p)) != anyR {
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
			if s.restrFilterMatch(k, &fl, c, p&1, og) {
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

// addRestrSrc is addRestr for mana source sl, whose chosen creature type the
// batch keeps when restriction f reads one.
func (s *State) addRestrSrc(k *prep, p uint8, sl Slot, f kop.FilterID, col int, n uint8) bool {
	return s.addRestrSrcV(k, p, sl, f, 0, col, n)
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
			ch = s.perms[sl].chosen
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
	m := s.restrSpendable(k, p, c, og)
	if m == 0 {
		return cost
	}
	return s.restrShortAdm(p, m, cost)
}

// restrShortAdm is cost less p's restricted mana in the batches of mask m,
// known spendable on it: each colour's pips, then generic.
func (s *State) restrShortAdm(p, m uint8, cost kop.ManaCost) kop.ManaCost {
	var t [kop.NumManaKinds]uint8
	s.rbUnits(p, m, &t)
	var n uint8
	for c, u := range t {
		d := min(u, cost.Pips[c])
		cost.Pips[c] -= d
		n += u - d
	}
	cost.Generic -= min(n, cost.Generic)
	return cost
}

// restrAffords: av (p's pool and sources) lacks cost, but p's restricted
// mana makes it up for the cast of card c, which may spend it (gorge's
// pool for that payment, AvailableFor, holds it).
//
//go:noinline
func (s *State) restrAffords(k *prep, p uint8, c CardID, og restrOrigin, cost *kop.ManaCost, av *avail) bool {
	t := *av
	return s.restrAvail(k, p, c, og, &t) && t.affords(cost, pipMaskOf(cost), 0)
}

// payRestr is pay for p's cast of object o while restricted mana floats:
// a spell that may spend it sees it in its pool and spends it before the
// ordinary mana of its colour (gorge emitRestrictedManaSpend carves the
// matching batches first); otherwise it stays beside the pool.
//
//go:noinline
func (s *State) payRestr(k *prep, p uint8, cost *kop.ManaCost, x int, o Obj) {
	s.payRestrRider(k, p, cost, x, o, false)
}

// payRestrRider is payRestr with the may-play any-colour rider.
func (s *State) payRestrRider(k *prep, p uint8, cost *kop.ManaCost, x int, o Obj, anyColor bool) {
	og := s.restrOriginOfObj(o)
	if e := s.spellEntryOf(o); e != nil {
		og = restrOriginOfEntry(e) // the push is behind the payment: the honest read
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
// ManaSymbol): engine order there.
func (s *State) scriptGapTap(k *prep, p uint8, a Action, sl Slot, restr, anyR, tapsDef slotMask, c int, spill *[kop.NumManaKinds]uint8) {
	s.tapGapChoice(k, p, sl, restr, anyR, tapsDef, s.scriptEngineAnswer(k, a, sl, anyR, tapsDef, c), spill)
}

// scriptEngineAnswer maps the script search's answer index c of gap sl to
// the engine-order answer tapGapChoice decodes (scriptGapTap).
func (s *State) scriptEngineAnswer(k *prep, a Action, sl Slot, anyR, tapsDef slotMask, c int) int {
	if !anyR.has(sl) && !tapsDef.has(sl) {
		return c
	}
	var mask uint8
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
	} else {
		mask = uint8(s.manaPCGen(k, sl).manaCol)
	}
	var order [kop.NumManaKinds]uint8
	no := 0
	for col := uint8(0); col < kop.NumManaKinds; col++ {
		if mask&(1<<col) != 0 {
			order[no] = col
			no++
		}
	}
	if c >= no {
		return off + c // beyond the mask's colours: engine order as before
	}
	needFirst(order[:no], s.playNeed(k, a)) // scriptOrderAnswers on the ask
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
			spill[bits.TrailingZeros8(uint8(a.ManaColors))] += max(a.ManaN, 1)
		}
		return
	}
	s.perms[sl].flags |= pfTapped
	s.touchSl(sl)
	s.manaTapped(k, sl)
	if tapsDef.has(sl) {
		s.tapDeferFloat(k, p, sl, c)
		return
	}
	if ab := &cd.Abilities[k.gen.anyR(s.cardAt(sl))]; restrWideAnyMana(ab) {
		if s.wideSecond(k, sl) { // its first ability's colours, then the multisets (mana_restrictedcombo.go)
			s.addExtra(k, p&1, sl, s.manaPCGen(k, sl)) // either tap brings its TapsForMana extra (gorge addTapExtra)
			if c < kop.NumManaKinds-1 {
				s.pl[p].pool[c]++
				return
			}
			c -= kop.NumManaKinds - 1
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

// scriptDFS is gorge's prefix search (scriptSearch.visit) for play a: p's
// gaps tapped in battlefield order (order[i:]), a Courtyard branched on its
// six answers, depth first; at each node after the first tap the walk
// (restricted mana priced as ordinary) is asked for a. It returns the
// number of gaps the found script taps (path holds their answers) and,
// in dead, whether the oracle cannot complete it: restricted mana floats
// and the play's pool does not pay it (the planner refuses that pool).
// Mana only adds up, so skipping a gap never prices what tapping it does
// not, and the chain with its branches is the search's order.
//
// The walk is iterative over one copy of s (a recursive walk's per-node
// copy escapes to the heap): each node rebuilds the copy from s by
// replaying path's taps, so the play loop allocates nothing.
func (s *State) scriptDFS(k *prep, a Action, p uint8, order []Slot, restr, anyR, tapsDef slotMask, path []uint8, dead *bool) (int, bool) {
	var d Decision
	depth := 0 // gaps tapped at this node: order[:depth], answers path[:depth]
	for {
		if depth > 0 {
			t := s.fork()
			t.g.noGrant = a.Kind() == AkActivate // the activation's granted mana is the grant search's
			var spill [kop.NumManaKinds]uint8
			for j := 0; j < depth; j++ {
				t.scriptGapTap(k, p, a, order[j], restr, anyR, tapsDef, int(path[j]), &spill)
			}
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
			found := hasOption(&d, a) && (t.npend == 0 || depth == len(order) || !resolve)
			if found && t.npend != 0 {
				t.resolvePrefixStack(k, t.sp, resolve)
				if t.over {
					found = false
				} else if resolve {
					t.restrOptions(k, &d, p)
					found = hasOption(&d, a)
				}
			}
			if found {
				*dead = false
				if t.rbAny(p&1) || spill != ([kop.NumManaKinds]uint8{}) {
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
		if depth < len(order) { // the node's first child
			path[depth] = 0
			depth++
			continue
		}
		for { // a leaf: the deepest gap with an answer left, else done
			if depth == 0 {
				return 0, false
			}
			j := depth - 1
			nch := uint8(1)
			switch {
			case anyR.has(order[j]):
				nch = 6
				if cd := &k.t.Cards[s.cardAt(order[j])]; restrWideAnyMana(&cd.Abilities[k.gen.anyR(s.cardAt(order[j]))]) {
					nch = wideAnswers
					if s.wideSecond(k, order[j]) {
						nch = wide2Answers
					}
				}
			case tapsDef.has(order[j]):
				nch = s.deferGapAnswers(k, order[j])
			}
			if path[j]+1 < nch {
				path[j]++
				break
			}
			depth--
		}
	}
}

// scriptDeadEndAny is scriptDeadEnd with a Courtyard or a Taps-deferred
// source among the gaps.
func (s *State) scriptDeadEndAny(k *prep, a Action, p uint8, restr, anyR, tapsDef slotMask) bool {
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef), order[:], 0)
	var path [MaxPerm]uint8
	dead := true
	if _, ok := s.scriptDFS(k, a, p, order[:n], restr, anyR, tapsDef, path[:], &dead); !ok {
		return true
	}
	return dead
}

// scriptPrefixAny is scriptPrefix with a Courtyard or a Taps-deferred source
// among the gaps: the search's script, played.
func (s *State) scriptPrefixAny(k *prep, a Action, p uint8, restr, anyR, tapsDef slotMask) {
	var d Decision
	s.plainOptions(k, &d, 0)
	if hasOption(&d, a) {
		return
	}
	var order [MaxPerm]Slot
	n := s.battlefieldOrder(p, s.scriptGaps(k, p).or(restr).or(anyR).or(tapsDef), order[:], 0)
	var path [MaxPerm]uint8
	var dead bool
	m, ok := s.scriptDFS(k, a, p, order[:n], restr, anyR, tapsDef, path[:], &dead)
	if !ok {
		return
	}
	var spill [kop.NumManaKinds]uint8
	for i := 0; i < m; i++ {
		s.scriptGapTap(k, p, a, order[i], restr, anyR, tapsDef, int(path[i]), &spill)
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
	s.availFor(k, p, &av)
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
	if s.rbAny(p&1) && s.bld().e.kind == seSpell {
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
			cost = s.restrShort(k, p, s.bld().e.card, s.restrOriginOfObj(s.bld().e.obj), cost)
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
			x = max(0, av.maxXAnyColor(&cost))
		} else {
			x = max(0, av.maxX(&cost, pipMaskOf(&cost)))
		}
		// the search is bounded by the mana the pool and every source can
		// hold, over the X count, before any reduction (rules
		// potentialModeCastPlan: hi = paymentPlanCensusTotal / base.X): a
		// reduction paying into {X} cannot lift the witness past it
		x = min(x, av.total/int(cost.X))
		// the restricted sources the payment may spend (the Emperor of
		// Palamecia's {U} or {R}) are census sources of gorge's witness
		// too: tapForRider's plan taps them, so count them and cap on the
		// same restricted-inclusive total
		if k.restrSrc {
			d := s.payDescNow(excl)
			t, multi, nm := s.restrSrcAvail(k, p, d, &av, NoSlot)
			x = restrWitnessX(x, t, multi[:nm], &cost)
		}
	}
	if cost.X != 0 && s.bld().e.kind == seSpell && s.xWitnessEats(k, p, s.bld().e.card, &cost, x, excl) {
		s.lowerXEats(k, p, &cost, x, excl, anyColor)
		return
	}
	untapped := s.untappedRestrSrcs(k, p)
	if !k.restrSrc || !k.t.Cards[s.bld().e.card].HasAltAdd && cost.Hyb[0] == 0 && !s.anyRestrEach(k, untapped) {
		if !s.tapForSpellTaps(k, p, &cost, x, excl, anyColor) {
			s.tapForRider(k, p, &cost, x, excl, anyColor)
		}
		return
	}
	pool := s.pl[p].pool
	var pk planner
	s.tapForPK(k, p, &cost, x, excl, false, anyColor, &pk)
	s.witnessManaRestricted(k, p, &pool, untapped, &pk)
}

// restrWitnessX is the largest-X witness's X once the restricted sources the
// payment may spend are counted: t holds the plain units and the restricted
// ones that make fixed mana, multi each restricted evaluated amount (Helga's
// power: that many mana of one colour). The census total X divides into
// counts the amounts too, as gorge's witness does (rules
// potentialModeCastPlan: paymentPlanCensusTotal / base.X), so Helga's two
// beside three lands lift {X}{U} to X = 4, not the lands' 3.
func restrWitnessX(x int, t avail, multi []uint8, cost *kop.ManaCost) int {
	tot := t.total
	for _, n := range multi {
		tot += int(n)
	}
	return min(max(x, t.maxXMulti(cost, pipMaskOf(cost), multi)), tot/int(cost.X))
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

// lkiCtrl is the controller of the resolving entry's first target o, which
// has left since it was targeted (once: ver is its targeted ver + 1), as
// it last existed (CR 608.2h; gorge's TargetedController): a permanent
// target's departed battlefield record (the newest one naming o), else o's
// owner. Stroke of Midnight's token for the destroyed permanent's
// controller (pair 84472), Condemn's life gain.
func (s *State) lkiCtrl(k *prep, o Obj) uint8 { return s.lkiCtrlSlot(k, o, 0) }

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
	if z == kop.ZoneGraveyard && s.g.on && s.graveRedirect(k, s.objs[o].card, z) == kop.ZoneLibrary {
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
func (s *State) divPicks(e *stackEntry, slot uint8, codes *[maxXTgt + 1]uint8) int {
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
		var codes [maxXTgt + 1]uint8
		n := s.divPicks(e, uint8(ti), &codes)
		tot := int(t.Divide)
		if t.Divide == kop.DivLKIP1P1 {
			tot = s.divLKITotal(e) // ctr_divided.go
		} else if t.Divide == kop.DivMaxV {
			tot = s.divMaxVTotal(k, t, e) // ctr_divided.go
		} else if t.Divide == kop.DivValue {
			// a value total (Ureni's land count): gorge divisionTotalFor
			// resolves DividedAsYouChoose$ as the ability is put on the stack
			tot = s.evalV(k, t.DivideV, s.ctxEntry(e))
			if tot < 0 {
				tot = 0
			}
			if tot > 255 {
				// the division arrays hold one uint8 per target (xTargets.div)
				s.capFull(StallXTgt)
				tot = 255
			}
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
	var codes [maxXTgt + 1]uint8
	n := s.divPicks(e, s.bld().divSlot, &codes)
	d.Kind, d.Sub, d.Player = DkChoose, ChCastDivide, e.ctrl
	for i := 0; i < n; i++ {
		d.add(MakeAction(AkChoose, codes[i], 0, 0))
	}
}

// answerCastDivide gives target c one more point.
func (s *State) answerCastDivide(k *prep, c uint8) {
	e := &s.bld().e
	var codes [maxXTgt + 1]uint8
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
	var codes [maxXTgt + 1]uint8
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
	c := *cost
	tb := s.buildCost0(k).Hyb[0] == kop.HybTwobrid
	if g := cost.XGeneric(x); g != 0 {
		// the window reads Generic, not X: fold the announced X in
		c.Generic += uint8(g)
	}
	ok, known := s.windowDry(k, p, &c, anyColor)
	if !known {
		probe := s.fork() // a gap or sacrificed source: the rare full probe
		ok, _ = probe.windowTapRider(k, p, &c, true, anyColor)
		s.unfork()
	}
	if ok {
		_, held := s.windowTapRider(k, p, &c, true, anyColor)
		for col := range held {
			s.pl[p].pool[col] += held[col]
		}
		return true
	}
	if known && k.gen != nil && k.gen.blgOn {
		return s.hybStrand(k, p, &c, anyColor)
	}
	if tb {
		// gorge's cast planner refuses a twobrid cost (pay.PlanCostDetail
		// "cost:twobrid"), so the announced cost is always paid through the
		// manual window this walk models, never re-planned with a witness.
		// A flexible source the walk tapped for an earlier owed colour can
		// strand a later pip only it could make (Frontier Bivouac's {U}
		// leaves a {R} pip no source makes); gorge reverses the cast and
		// keeps the tap and its mana (CR 601.2g-h, 733.1), so the kernel
		// does too.
		return s.hybStrand(k, p, &c, anyColor)
	}
	var pk planner
	pk.bfl, pk.try = true, true
	s.tapForPK(k, p, cost, x, exclude, false, anyColor, &pk)
	if pk.needT > 0 && !pk.found {
		return s.hybStrand(k, p, &c, anyColor)
	}
	return true
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
		if e.flags&sfFree != 0 && cost != (kop.ManaCost{}) || s.bld().bfl&bflLowered != 0 || e.trigPl == trigPlMadness && debugMadnessMutate == 0 || e.trigPl == trigPlMiracle {
			return false
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
		anyColor := s.bld().bfl&bflAnyColor != 0
		if (e.flags&sfAnyMana != 0 || anyColor) && s.bld().bfl&bflMsDone == 0 {
			s.tapRiderPlan(k, *e) // (not again after a tap-another pick: the window runs on)
		}
		if k.gen != nil && k.gen.anyConvoke && s.tapPaysEntry(k, e) && s.bld().bfl&bflTargetPaid != 0 && s.convokePickCount() == 0 {
			// a target-paid convoke cast with no creature announced keeps its
			// plan at the printed cost (state.go bflTargetPaid): the planner
			// pays, the Treasure for the colour the cost owes
			s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
		} else if k.gen != nil && (k.gen.anyConvoke && s.tapPaysEntry(k, e) || s.delvePaysEntry(k, e)) {
			// the announced creatures pay their share before the plan runs
			// (CR 702.51a; gorge convokeAsk before manaWindowAsk) and are
			// withheld from the planner as mana sources. Gorge's planner
			// refuses a convoke cast (payment_plan.go "shape:contribution"),
			// so the payment runs through the manual CR 601.2g window: the
			// first untapped source, repeat until the pool pays.
			occ := s.occ
			s.occ = s.occ.andNot(s.convokeHidden())
			raw := xFolded(cost, int(e.x))
			cost = s.convokeApplyPicks(k, raw)
			if s.bld().bfl&bflMsDone == 0 && s.paymentStranded(k, e, &cost) {
				s.occ = occ
				s.releaseCostTaps(k, &saved)
				s.reverseCast(k)
				return true
			}
			s.windowTapUseful(k, e.ctrl, &cost, &raw, true, anyColor, s.bld().pipN == 0, false)
			s.occ = occ
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
			if s.phyCastPlanSettles(k, e, half, cost) {
				s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
			} else {
				s.phyPay(k, e.ctrl, &cost, int(e.x), e.obj)
			}
		} else if s.bld().pipN != 0 {
			// a hybrid pip was announced: gorge's cast planner refuses a
			// hybrid cost (pay.PlanCostDetail "cost:hybrid"), so the cast is
			// paid through its manual CR 601.2g window, which activates
			// sources in battlefield order (pay.ManaSourceIDs), not the
			// planner's ObjID-ordered witness (pay.CompareSteps).
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
			// gorge's plan does.
			if cd.BeholdN == 0 || s.altAddFreePart(k, e.ctrl, e.card, e.obj) {
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
				if s.rbAny(e.ctrl) {
					// the plan taps only for what the admitted restricted mana
					// leaves (pay.PlanPool), as the plain branch below does
					if m := s.restrSpendable(k, e.ctrl, e.card, restrOriginOfEntry(e)); m != 0 {
						cost = s.restrShortAdm(e.ctrl, m, cost)
					}
				}
				s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
			}
		} else {
			if s.rbAny(e.ctrl) {
				// a rider plan that tapped a wide restricted source floated its
				// mana (State.rb): the payment spends the batches this cast admits
				// first (payRestrSpend), so the plan taps only for what the
				// restricted mana leaves (gorge pay.PlanPool, pay/plancost.go)
				if m := s.restrSpendable(k, e.ctrl, e.card, restrOriginOfEntry(e)); m != 0 {
					cost = s.restrShortAdm(e.ctrl, m, cost)
				}
			}
			s.tapForRider(k, e.ctrl, &cost, int(e.x), e.obj, anyColor)
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
			full := a.Cost.Mana
			ok, held := s.windowTapRiderUseful(k, e.ctrl, &cost, &full, true, false)
			if !ok {
				// gorge's fallback window taps as it walks and leaves the
				// mana abilities' taps and floated mana behind when payCast
				// then aborts the activation (CR 733.1): a tap cannot be
				// undone. Keep the window's state, unlike a dry probe.
				s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
				return true
			}
			for c := range held {
				s.pl[e.ctrl].pool[c] += held[c]
			}
		case k.gen != nil && k.gen.anyPowerUp && a.Cost.Flags&kop.CostPowerUp != 0 && s.powerUpPlanRejected(k, a):
			// The planner's printed-cost witness is outdated. gorge arms it
			// whenever the charge still owes generic or coloured mana, hybrid
			// symbols or not (armAbilityAutoPay), and re-arms for the reduced
			// cost, tapping its plan's order. A charge the reductions left
			// hybrid-only arms no witness at all (powerUpArmsNone), so the
			// manual window pays source by source, first useful one in
			// battlefield order, tapping nothing when the pool already pays
			// it.
			cc := s.buildCostCharge(k)
			w := s.powerUpWitness(k, a, sl, e.ctrl)
			if !s.powerUpArmsNone(k) && s.abilityPlanSettles(k, e, w, w, cc, &saved) {
				s.tapFor(k, e.ctrl, &cc, int(e.x), NoObj)
			} else if !s.windowTapHeldOK(k, e.ctrl, &cc) {
				// the window ran out of sources: gorge aborts the activation
				// (CR 733.1) and keeps the taps and the floated mana, as for
				// the waterbend fallback above (mana_gate.go)
				s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
				return true
			}
		case k.phyMana && s.phyBuild(k):
			// a Phyrexian activation: the plan for the cost its pips settled,
			// armed on the mana half; the manual window when none exists
			// (mana_phyrexian.go)
			half := a.Cost.Mana
			half.Tb, half.Hyb = 0, [len(kop.ManaCost{}.Hyb)]kop.ColorMask{}
			if s.abilityPlanSettles(k, e, half, half, cost, &saved) {
				s.tapFor(k, e.ctrl, &cost, int(e.x), NoObj)
			} else {
				s.phyPay(k, e.ctrl, &cost, int(e.x), NoObj)
			}
		case s.bld().pipN != 0 && s.hybAbilityOffPlan(k, e.ctrl, &cost) &&
			!s.abilityPlanSettles(k, e, s.hybResolve(k, e.ctrl, s.abPlannedCost(k, a, sl, e.ctrl), 0), s.hybResolve(k, e.ctrl, s.abPlannedCost(k, a, sl, e.ctrl), 0), cost, &saved):
			// gorge has no plan for the cost the pips settled: its manual window, which can run dry
			// when a source's first ability is not the colour owed (a
			// granted basic land type: static_landtype.go; a filter's
			// free {C}); gorge then aborts the activation and keeps the
			// taps, as for power-up
			if !s.windowTapHeldOK(k, e.ctrl, &cost) && k.gen != nil && (k.gen.blgOn || k.gen.anyFilt) {
				s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
				return true
			}
		case a.Cost.Flags&kop.CostSac != 0 && k.sacMana:
			s.tapForSacKept(k, e.ctrl, &cost, int(e.x), a, sl)
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
			ok := s.windowTapHeldOK(k, e.ctrl, &cost)
			if !s.winPaused() {
				s.bld().bfl &^= bflMsDone
				if !ok {
					s.reverseActivation(k, &saved, hostSl, sl, selfUntapped)
					return true
				}
			}
		default:
			s.tapFor(k, e.ctrl, &cost, int(e.x), NoObj)
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
	if k.gen == nil || s.bld().pickN == 0 || !s.committedPicks() || !s.windowCensusComplete(k, e) {
		return false
	}
	anyColor := s.bld().bfl&bflAnyColor != 0
	ok, known, restr := s.windowDryRestr(k, e.ctrl, cost, anyColor)
	if !known && restr {
		saved := s.fork()
		defer s.unfork()
		c := *cost
		ok, _ = s.windowTapRider(k, e.ctrl, &c, false, anyColor)
		s.CopyFrom(saved)
		return !ok
	}
	return known && !ok
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
		if was == 0 && k.gen != nil && s.subs[kop.TrigTaps].any() {
			s.fireTaps(k, sl)
		}
	}
	if f&kop.CostUntapSelf != 0 && sl != NoSlot {
		s.untapCost(k, sl)
	}
	if f&kop.CostOncePerTurn != 0 && e.ab < 8 {
		s.perms[sl].actUsed |= 1 << e.ab
	}
	if f&kop.CostLoyalty != 0 {
		s.perms[sl].flags |= pfLoyaltyUsed
	}
	if f&kop.CostLoyaltyX != 0 && sl != NoSlot {
		s.payCounterX(sl, int(a.Cost.Ctr), e.x) // [-X] loyalty or SubCounter<X/KIND>
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
		s.perms[sl].subCtrN(s, int(a.Cost.Ctr), int(a.Cost.N))
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
		s.payLifeCost(k, e.ctrl, &a.Cost, e.x)
	}
	if f&kop.CostPayEnergy != 0 {
		s.payEnergy(e.ctrl, &a.Cost)
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
// prep.tapMana; an activation's own window and a probe on a fork still take the first candidate.

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
		// an activation's window lists them only beside a sacrifice-another
		// source, the one case it is modelled for (sacXWindowPays)
		return b.stage == bsMana && b.bfl&bflManaStep != 0 && s.sacXLive(k, b.e.ctrl)
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
	if !k.tapMana || DebugNoSacXMana || cost.X != 0 || !s.sacXLive(k, e.ctrl) {
		return false
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
