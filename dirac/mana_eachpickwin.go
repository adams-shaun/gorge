package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An all-at-once source (Azorius Chancery's {W}{U}) under one reflect carrier
// (Mana Flare) tapped by the cast window's real walk. gorge's window answer
// activates the source and the carrier's trigger resolves in the same batch
// (CR 605.3b): its ManaReflected body asks the colour of the one more mana, over
// the types the tap produced (CR 106.12a; engine/effects ManaReflected, the
// candidates in W U B R G order), a decision of its own. srcEachPick prices the
// pick for the census; the walk poses it, with the tap's production already
// floated, and the answer floats the pick (applyCastWinTap, eachPickWinDo). A
// probe, a fork or a census walk answers no ask and keeps the stall
// (addExtraCol).

// eachPickWinCols is the colours the pick of the source at sl is asked over
// (W U B R G bit order), 0 unless the ask has a choice to make: a single type
// is no ask, and a {C}-bearing production's candidates depend on the carrier's
// widening, which this arm does not model.
func (s *State) eachPickWinCols(k *prep, sl Slot) uint8 {
	if k.gen == nil || !k.gen.anyTapExtra {
		return 0
	}
	pc := s.manaPCGen(k, sl)
	if s.srcEachPick(k, sl, pc) != 1 {
		return 0
	}
	m := eachPickMask(pc)
	if m&^uint8(kop.ColAny) != 0 || m&(m-1) == 0 {
		return 0
	}
	return m
}

// eachPickWinAsk reports whether the window's source sl is the one whose pick
// the build is waiting on: tapped (the walk taps it before it pauses; a
// tap-another source's pick pauses before its tap) and no Condition-split arm.
func (s *State) eachPickWinAsk(k *prep, sl Slot) bool {
	return s.perms[sl].flags&pfTapped != 0 && !s.splitWinColAsk(k, sl) && s.eachPickWinCols(k, sl) != 0
}

// windowAddExtra floats the extra mana of the walk's tap of sl, which made
// colour col (addExtraCol), and reports whether the pick of an all-at-once
// source is left to be asked: only a live cast's real walk poses it (no
// census, no potential walk, no fork), and only for a plain unit -- a painful,
// sacrificed or untapped one keeps the stall.
func (s *State) windowAddExtra(k *prep, p uint8, sl Slot, pc *pcard, col int, census, potential, sacOnly, noTap bool) bool {
	if census || potential || sacOnly || noTap || pc.painCol != 0 || pc.manaSac || !s.buildLive() || s.isFork() ||
		s.eachPickWinCols(k, sl) == 0 || s.srcEachPick(k, sl, pc) != 1 {
		s.addExtraCol(k, p, sl, pc, col)
		return false
	}
	s.addGatedExtra(k, p, sl, s.perms[sl].ctrl) // the gated extra does not depend on the pick
	xv := s.srcExtra(k, sl, pc)                 // the production beside the unit's own colour
	s.addXv(p, &xv)
	return true
}

// eachPickWinPause poses the pick: the build waits on it the way it waits on
// the tap-another pick (g.winSrc, bflWinTap); the answer floats the pick and
// the stage runs the window on (applyCastWinTap).
func (s *State) eachPickWinPause(sl Slot, col int) {
	s.g.winSrc, s.g.winCol = sl, uint8(col)
	if s.bld().stage == bsPay {
		s.bld().bfl |= bflWinTap | bflPayDone
	} else {
		s.bld().bfl |= bflWinTap | bflMsDone
	}
}

// eachPickWinOptions fills the pick's ask: gorge's mana_color choice
// (ChGenManaChoice, mode = the colour), the produced colours in W U B R G order.
func (s *State) eachPickWinOptions(k *prep, d *Decision) {
	d.Kind, d.Sub, d.Player = DkChoose, ChGenManaChoice, s.bld().e.ctrl
	m := s.eachPickWinCols(k, s.g.winSrc)
	for c := uint8(0); c < 5; c++ {
		if m&(1<<c) != 0 {
			d.add(MakeAction(AkChoose, 0, 0, c))
		}
	}
}

// eachPickWinDo floats the answer's one more mana of the picked colour.
func (s *State) eachPickWinDo(p uint8, a Action) {
	s.bld().bfl &^= bflWinTap
	s.poolAdd(p, int(a.Mode()), 1)
}
