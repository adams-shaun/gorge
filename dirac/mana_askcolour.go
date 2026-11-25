package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// A colour-asking mana source (kop.Ability.ManaAskColour: "{T}: Choose a
// color. Add an amount of mana of that color equal to your devotion to that
// color"). gorge resolves the chain off the stack
// (engine/rules mana_activation.go withOffStackMana, askOffStackMana,
// answerNestedChooseColor): the activation asks one colour of WUBRG, records
// it on the source, adds ManaV mana of it in one unit (Count$Devotion.Chosen,
// kop.ExDevotionChosen), and the Cleanup clears the record before the
// off-stack frame closes, so the next activation asks again.
//
// The planner fails closed on an unrecorded Chosen (pay.PaymentPlanChoiceColours
// returns nil), so the census never prices the source and autopay never taps
// it: it joins the Taps-deferred route (planDeferred, mana_tapsdefer.go), and
// only a potential play's prefix search or the manual window activates it,
// the {T} paid before the ask. A live cast's real window poses the ask
// (askColPause, answered by applyCastWinAsk); a census walk, a potential walk
// and a fork answer it silently (askColSilent), as the Condition-split arm does
// (mana_condsplit.go splitWinColArm).

// prepManaAsk marks card id when its first mana ability asks a colour (prepareGen,
// once per card).
//
//go:noinline
func prepManaAsk(g *genPrep, c *kop.CardDef, id CardID, pc *pcard) {
	j := int(pc.manaAb)
	if j < 0 || j >= len(c.Abilities) {
		return
	}
	a := &c.Abilities[j]
	if a.Kind != kop.AbMana || !a.ManaAskColour || a.AFlags&kop.AfLatent != 0 || !pc.manaTap {
		return
	}
	g.smallFor(id).askAb = int8(j)
	g.anyCondSplit, g.anyTapsDefer = true, true // the deferred route's gates (deferLive, kw_madness)
}

// askColDeferred: sl is a colour-asking mana source the planner defers.
func (s *State) askColDeferred(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.anyCondSplit {
		return false
	}
	r := k.gen.small(s.cardAt(sl))
	return r != nil && r.askAb >= 0 && !s.genLost(k, sl)
}

// askColAb is the colour-asking ability of the deferred source at sl.
func (s *State) askColAb(k *prep, sl Slot) *kop.Ability {
	c := s.cardAt(sl)
	return &k.t.Cards[c].Abilities[k.cards[c].manaAb]
}

// askColUsable: the source at sl can be activated now (scriptTapsDefer).
func (s *State) askColUsable(k *prep, sl Slot) bool { return s.splitUsable(k, sl) }

// askColValue is the mana an activation of sl makes if its answer is col: ManaV
// read with col recorded on the source. A pricing read, so the record is
// restored by hand and nothing is marked touched.
func (s *State) askColValue(k *prep, sl Slot, col uint8) int {
	old := s.perms[sl].chosen
	s.perms[sl].chosen = objWord(col)
	n := s.dynManaN(k, s.askColAb(k, sl), sl)
	s.perms[sl].chosen = old
	return n
}

// askColUnits prices the gap of the source at sl: one relaxed unit, the
// colours an answer makes mana of, as many as the best answer makes.
func (s *State) askColUnits(k *prep, sl Slot, av *avail) {
	mask, best := uint8(0), 0
	for col := uint8(0); col < 5; col++ {
		if n := s.askColValue(k, sl, col); n > 0 {
			mask |= 1 << col
			best = max(best, n)
		}
	}
	if mask != 0 {
		av.addUnits(mask, uint8(min(best, maxDynMana)))
	}
}

// askColBoundUnits is the source's unit in the potential bound and in the
// prefix search's relaxed gate (addTapsDeferUnits, availPotentialFor).
// gorge's PotentialMana reads the AB$ ChooseColor head's own Mana params
// (params.ManaOf: no Produced$, no Amount$), one {C}, so a play only the
// source's devotion mana pays is never searched. Another source opens the
// bound in its colours (an Amount$ X source, a Produced$ Any one:
// potentialUnbounded), and gorge's search then reaches the answers of those
// colours at their real amount (askColValue).
func (s *State) askColBoundUnits(k *prep, sl Slot) (uint8, uint8) {
	open := s.askColOpenMask(k, sl)
	mask, best := uint8(0), 0
	for col := uint8(0); col < 5; col++ {
		if open&(1<<col) == 0 {
			continue
		}
		if n := s.askColValue(k, sl, col); n > 0 {
			mask |= 1 << col
			best = max(best, n)
		}
	}
	if mask == 0 {
		return uint8(kop.ColC), 1
	}
	return mask, uint8(min(best, maxDynMana))
}

// askColBoundUnit is askColBoundUnits added to a's prefix-search census.
func (s *State) askColBoundUnit(k *prep, sl Slot, av *avail) {
	mask, n := s.askColBoundUnits(k, sl)
	av.addUnits(mask, n)
}

// askColPotential folds the colour-asking source at sl into the potential
// walk's bound as askColBoundUnits reads it: one {C} unless another source
// opens the bound. gorge's potential never prices the head's devotion mana on
// its own, so an open any-colour unit here offers a play its planner does not
// (the kicked-cast over-offer).
func (s *State) askColPotential(k *prep, sl Slot, pc *pcard, pain, life int, add, fold func(Slot, uint8, uint8), cen *potCensus) {
	mask, n := s.askColBoundUnits(k, sl)
	painDrop, painAdd := cen.painGate(pain, life, mask, add, fold)
	painAdd(sl, mask, n)
	if painDrop == 0 {
		cen.tapRec(sl, mask, n)
	}
	cen.pain = 0
	s.potentialTapExtraGated(k, sl, pc, mask, fold, cen, painDrop != 0)
}

// askColOpenMask is the colours another usable source of sl's controller
// prices unbounded in gorge's PotentialMana: a dynamic amount (pcard.manaDyn)
// in its colours, an any-colour production in all five.
func (s *State) askColOpenMask(k *prep, sl Slot) uint8 {
	open := uint8(0)
	for m := s.ctrlM[SeatIx(s.perms[sl].ctrl)].and(s.occ); m.any(); m = m.rest() {
		o := m.first()
		if o == sl || !s.canTapForMana(k, o) {
			continue
		}
		pc := s.manaPCGen(k, o)
		if mask := s.srcMaskAt(pc, o); pc.manaDyn || mask&uint8(kop.ColAny) == uint8(kop.ColAny) {
			open |= mask
		}
	}
	return open & uint8(kop.ColAny)
}

// askColDo is the answer: record col on the source, float ManaV mana of it in
// one unit, clear the record (gorge's DB$ Cleanup | ClearChosenColor$ True).
func (s *State) askColDo(k *prep, p uint8, sl Slot, col int) {
	s.recordChosenColor(sl, uint8(col))
	n := s.dynManaN(k, s.askColAb(k, sl), sl)
	s.clearChosenColor(sl)
	if n > 0 {
		s.poolAdd(p, col, n)
		s.addExtraCol(k, p, sl, s.manaPCGen(k, sl), col) // the tap's StatTapExtra mana floats beside it
	}
	s.artTag(k, p, sl, col)
}

// askColPause is windowTapWalk's live arm: only a live cast's real window poses
// the ask (no census, no potential walk, no fork); the source taps now and
// the build waits on the colour (splitWinColPause), the answer runs
// applyCastWinAsk.
func (s *State) askColPause(k *prep, sl Slot, census, potential bool) bool {
	if census || potential || !s.buildLive() || s.isFork() || !s.askColDeferred(k, sl) {
		return false
	}
	s.splitWinColPause(k, sl)
	return true
}

// windowPauseArm is windowTapWalk's pausing arm: a Condition-split source's colour
// ask (splitWinColArm) or a colour-asking source's (askColPause). One call, so the
// walk's own complexity stays under its ceiling.
func (s *State) windowPauseArm(k *prep, sl Slot, census, potential bool) bool {
	return s.splitWinColArm(k, sl, census, potential) || s.askColPause(k, sl, census, potential)
}

// windowFiltAskArm is windowTapWalk's silent arm: a hybrid-fee filter's lowest Combo
// answer (windowHybFiltArm) or a colour-asking source answered silently (askColSilent).
func (s *State) windowFiltAskArm(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, anyColor, noOwed, potential bool) bool {
	return s.windowHybFiltArm(k, p, sl, useful, noOwed, clamp, anyColor, potential) ||
		s.askColSilent(k, p, sl, cost, useful, clamp, anyColor, noOwed, potential)
}

// askColSilent is windowTapWalk's silent arm (a census walk, a potential walk,
// a fork): the source taps and the walk's own colour answer runs the
// activation; true when it did.
func (s *State) askColSilent(k *prep, p uint8, sl Slot, cost, useful *kop.ManaCost, clamp, anyColor, noOwed, potential bool) bool {
	if !s.askColDeferred(k, sl) {
		return false
	}
	pc := s.windowGrantPC(k, sl)
	col := s.windowWalkCol(k, p, cost, useful, sl, pc, clamp, anyColor, noOwed, potential)
	s.windowTapCost(k, sl, false)
	s.askColDo(k, p, sl, min(col, 4))
	return true
}

// askColOptions poses the ask: the colours W U B R G in order.
func askColOptions(d *Decision) {
	d.Sub = ChGenManaChoice
	for c := uint8(0); c < 5; c++ {
		d.add(MakeAction(AkChoose, 0, 0, c))
	}
}

// applyCastWinAsk answers the cast window's colour ask of the source it paused on.
func (s *State) applyCastWinAsk(k *prep, a Action) {
	b := s.bld()
	b.bfl &^= bflWinTap
	s.askColDo(k, b.e.ctrl, s.g.winSrc, int(a.Mode()))
}

// askColDeferAnswers is how many colour answers the prefix search poses:
// gorge's ask offers W U B R G whatever the devotion (an answer of a colour
// with no devotion taps the source for nothing).
func (s *State) askColDeferAnswers(k *prep, sl Slot) uint8 { return 5 }

// askColDeferFloat answers the prefix search's c'th answer: colour c of
// W U B R G (scriptEngineAnswer keeps gorge's ask order).
func (s *State) askColDeferFloat(k *prep, p uint8, sl Slot, c int) {
	s.askColDo(k, p, sl, min(c, 4))
}
