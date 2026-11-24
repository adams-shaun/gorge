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

// askColDeferAnswers is how many colour answers the prefix search poses: one
// per colour the source makes mana of (askColUnits).
func (s *State) askColDeferAnswers(k *prep, sl Slot) uint8 {
	n := uint8(0)
	for col := uint8(0); col < 5; col++ {
		if s.askColValue(k, sl, col) > 0 {
			n++
		}
	}
	return max(n, 1)
}

// askColDeferFloat answers the prefix search's c'th colour (the colours
// askColUnits prices, in W U B R G order).
func (s *State) askColDeferFloat(k *prep, p uint8, sl Slot, c int) {
	for col := uint8(0); col < 5; col++ {
		if s.askColValue(k, sl, col) > 0 {
			if c == 0 {
				s.askColDo(k, p, sl, int(col))
				return
			}
			c--
		}
	}
	// unreachable: askColDeferAnswers bounds c
}
