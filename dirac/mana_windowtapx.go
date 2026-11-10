package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A tap-another-permanent mana source (Saruli Caretaker: "{T}, Tap an untapped
// creature you control: Add one mana of any color") in gorge's unless and ward
// mana window. The window lists it beside the plain sources, in battlefield
// order, and activating it asks which permanent to tap (a "tapcost" choose, only
// with two or more candidates, in battlefield order) and then the colour, which
// the harness's window answer takes as the lowest the cost still owes (windowCol).
// The payment planner cannot price that choice, so autopay never uses the source
// (mana_tapanother.go); the window does. The ask is the resolution's own
// (raGenWinTap, ChWindowTap), posed from inside the window walk: the walk stops
// there, and the answer taps, floats the mana and runs the walk on (payWindow).
//
// Only the unless, ward and trigger-cost windows that wait out a pick are modelled (winPay,
// winWard, winTrig); every other window leaves the source out as before. Everything is
// behind prep.tapMana.

// ChWindowTap: source = the permanent tapped to pay a window-activated
// tap-another-permanent mana source's cost (gorge's KChoose "tapcost").
const ChWindowTap ChoiceKind = 253

// raGenWinTap is the resolution ask of that pick (ChWindowTap).
const raGenWinTap = raGenManifestDread + 74

// winMode: which payment the window serves, so the answer can finish it.
const (
	winNone uint8 = iota
	winPay        // an unless or ward cost inside a program (raGenPay's PayMana)
	winWard       // a ward cost asked as the generic ward ask (raGeneric)
	winTrig       // a trigger's own cost funded through the manual window, which cannot pay it
)

// payWindowAsk is payWindow for a window that can wait out a tap-another pick;
// paused when it did, and the caller returns: the answer finishes the payment.
// short: the colours the window took left the cost unpaid.
//
//go:noinline
func (s *State) payWindowAsk(k *prep, p uint8, cost *kop.ManaCost, mode uint8) (paused, short bool) {
	if k.tapMana {
		s.g.winMode = mode
	}
	if paused, short = s.payWindow(k, p, cost); paused {
		return true, false
	}
	s.g.winMode = winNone
	return false, short
}

// trigWindowTap is windowTapNoOwed for a trigger's own cost funded through the manual
// window, which can wait out a tap-another pick (paused: the answer runs the window on).
//
//go:noinline
func (s *State) trigWindowTap(k *prep, p uint8, cost *kop.ManaCost) (paused bool) {
	if k.tapMana {
		s.g.winMode = winTrig
	}
	s.windowTapNoOwed(k, p, cost)
	if s.res.ask == raGenWinTap {
		return true
	}
	s.g.winMode = winNone
	return false
}

// windowTapXOffer: source o is a tap-another-permanent source the window lists
// and can take now (one permanent to tap), and its colours add something the
// clamp keeps. A second tap-another ability beside a plain first (Scene of the
// Crime, kop.SecondTapXManaAb) is not offered here: the window takes the
// source's plain first ability (windowCanTap), and its tap-another one is not
// modelled in the windows. Nor is a sacrifice-another source (Bolg's
// Company, kop.SacXEachManaAb): the window leaves it out.
//
//go:noinline
func (s *State) windowTapXOffer(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	if !k.tapMana || !s.scriptTapX(k, p).has(o) || k.tapXSecond(s.cardAt(o)) || k.sacXOf(s.cardAt(o)) {
		return false
	}
	return !decided || s.windowSrcMask(k, o)&want != 0
}

// winTapN is how many permanents source sl's cost taps: one (Saruli Caretaker)
// or two (Supportive Parents, whose window pick is gorge's one "choose 2", asked
// a permanent at a time).
func (s *State) winTapN(k *prep, sl Slot) int {
	return int(k.tapXAbOf(s.cardAt(sl)).Cost.N)
}

// winTapPick records pick a of the window's tap-another source; true when the
// source still owes its second permanent (the ask stays posed), else cand and
// cand2 are the permanents to tap, in pick order.
func (s *State) winTapPick(k *prep, a Action) (more bool, cand, cand2 Slot) {
	sl := s.objs[Obj(a.Source())].slot
	if s.winTapN(k, s.g.winSrc) == 2 {
		if s.g.winFirst == 0 {
			s.g.winFirst = uint8(sl) + 1
			return true, NoSlot, NoSlot
		}
		first := Slot(s.g.winFirst - 1)
		s.g.winFirst = 0
		return false, first, sl
	}
	return false, sl, NoSlot
}

// winTapOptions adds the window pick's options: the source's candidates in
// battlefield order, less a two-permanent source's first pick.
func (s *State) winTapOptions(k *prep, p uint8, d *Decision) {
	var cs [MaxPerm]Slot
	n := s.tapXCands(k, p, s.g.winSrc, &cs)
	for _, sl := range cs[:n] {
		if s.g.winFirst != 0 && sl == Slot(s.g.winFirst-1) {
			continue
		}
		d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
	}
}

// windowTapXGo activates the window's tap-another source sl: with one candidate
// it taps it and floats the mana; with several it poses the pick and reports
// true (the window walk stops).
//
//go:noinline
func (s *State) windowTapXGo(k *prep, p uint8, cost, useful *kop.ManaCost, sl Slot, clamp, anyColor, noOwed bool) bool {
	var cs [MaxPerm]Slot
	n := s.tapXCands(k, p, sl, &cs)
	need := s.winTapN(k, sl) // with exactly that many candidates gorge taps them unasked
	if n > need && s.g.winMode != winNone {
		s.g.winSrc = sl
		s.res.ask, s.res.askP = raGenWinTap, p
		return true
	}
	col := s.windowTapXCol(k, p, cost, sl, anyColor, noOwed)
	if s.g.winMode == winNone && noOwed && clamp && !anyColor {
		// the cast window's colour ask lists only what the clamp keeps (gorge paymentUsefulColours)
		if want, decided := s.windowUseful(k, p, useful); decided {
			if m := s.windowSrcMask(k, sl) & want; m != 0 {
				col = uint8(bits.TrailingZeros8(m))
			}
		}
	}
	// The altAdd abort window (mana_window.go altAddWindowTap) runs before
	// the mana stage and is synchronous: it has no build stage to resume
	// after a pick, so it takes the first candidates the oracle's
	// WindowAnswer would answer. CR 601.2g.
	if n > need && s.bld().stage != bsAltAdd && s.castWinTapPause(sl, col) {
		return true
	}
	cand2 := NoSlot
	if need == 2 {
		cand2 = cs[1] // a probe takes the first candidates
	}
	s.tapXDo(k, p, tapXStep{sl: sl, cand: cs[0], cand2: cand2, col: col})
	return false
}

// windowTapXCol is the colour the window's source sl makes (windowCol).
func (s *State) windowTapXCol(k *prep, p uint8, cost *kop.ManaCost, sl Slot, anyColor, noOwed bool) uint8 {
	pc := &k.cards[s.cardAt(sl)]
	if k.gen != nil {
		pc = s.manaPCGen(k, sl)
	}
	return uint8(s.windowCol(k, p, cost, sl, pc, anyColor, noOwed))
}

// winTapCost is the cost of the payment the paused window serves.
func (s *State) winTapCost(k *prep) kop.ManaCost {
	if s.g.winMode == winWard {
		return kop.ManaCost{Generic: uint8(s.res.e.trigAmt &^ wardAltBits)} // a discard-or-{N} ward's N (kw_wardcost.go)
	}
	return manaOf(int(s.g.pickLeft), s.numTimesPips(k, s.prog(k)[s.res.pc].B.N))
}

// askOptionsWinTap builds the pick's options: the source's candidates in
// battlefield order; false: not this ask.
func (s *State) askOptionsWinTap(k *prep, d *Decision) bool {
	if s.res.ask != raGenWinTap {
		return false
	}
	d.Sub = ChWindowTap
	s.winTapOptions(k, s.res.askP, d)
	return true
}

// applyAskWinTap taps the permanent picked and the source, floats the colour,
// and runs the window on; false: not this ask.
func (s *State) applyAskWinTap(k *prep, a Action) bool {
	if s.res.ask != raGenWinTap {
		return false
	}
	more, cand, cand2 := s.winTapPick(k, a)
	if more {
		return true // the second permanent is asked next
	}
	r := &s.res
	p := r.askP
	r.ask = raNone
	cost := s.winTapCost(k)
	sl := s.g.winSrc
	col := s.windowTapXCol(k, p, &cost, sl, false, true)
	s.tapXDo(k, p, tapXStep{sl: sl, cand: cand, cand2: cand2, col: col})
	if s.g.winMode == winTrig {
		// the trigger's window taps on; the charge fails, as payReplanned's does
		if s.trigWindowTap(k, p, &cost) {
			return true
		}
		s.payDone(k, false)
		return true
	}
	paused, short := s.payWindow(k, p, &cost)
	if paused {
		return true
	}
	mode := s.g.winMode
	s.g.winMode = winNone
	if mode == winWard {
		if idx := s.wardedIndex(&s.res.e); short && idx >= 0 {
			s.wardCounterAt(k, idx)
		}
		s.finishResolve(k, false)
	} else {
		s.payDone(k, !short)
	}
	return true
}
