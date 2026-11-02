package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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
// Only the unless and ward windows that wait out a pick are modelled (winPay,
// winWard); every other window leaves the source out as before. Everything is
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
)

// payWindowAsk is payWindow for a window that can wait out a tap-another pick;
// true when it did, and the caller returns: the answer finishes the payment.
//
//go:noinline
func (s *State) payWindowAsk(k *prep, p uint8, cost *kop.ManaCost, mode uint8) bool {
	if k.tapMana {
		s.g.winMode = mode
	}
	if s.payWindow(k, p, cost) {
		return true
	}
	s.g.winMode = winNone
	return false
}

// windowTapXOffer: source o is a tap-another-permanent source the window lists
// and can take now (one permanent to tap), and its colours add something the
// clamp keeps.
//
//go:noinline
func (s *State) windowTapXOffer(k *prep, p uint8, o Slot, decided bool, want uint8) bool {
	if !k.tapMana || !s.scriptTapX(k, p).has(o) {
		return false
	}
	c := s.cardAt(o)
	if k.t.Cards[c].Abilities[k.cards[c].manaAb].Cost.N != 1 {
		return false
	}
	return !decided || s.windowSrcMask(k, o)&want != 0
}

// windowTapXGo activates the window's tap-another source sl: with one candidate
// it taps it and floats the mana; with several it poses the pick and reports
// true (the window walk stops).
//
//go:noinline
func (s *State) windowTapXGo(k *prep, p uint8, cost *kop.ManaCost, sl Slot, anyColor, noOwed bool) bool {
	var cs [MaxPerm]Slot
	if n := s.tapXCands(k, p, sl, &cs); n >= 2 {
		s.g.winSrc = sl
		s.res.ask, s.res.askP = raGenWinTap, p
		return true
	}
	s.tapXDo(k, p, tapXStep{sl: sl, cand: cs[0], cand2: NoSlot, col: s.windowTapXCol(k, p, cost, sl, anyColor, noOwed)})
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
		return kop.ManaCost{Generic: uint8(s.res.e.trigAmt)}
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
	var cs [MaxPerm]Slot
	n := s.tapXCands(k, s.res.askP, s.g.winSrc, &cs)
	for _, sl := range cs[:n] {
		d.add(MakeAction(AkChoose, uint8(s.perms[sl].obj), 0, 0))
	}
	return true
}

// applyAskWinTap taps the permanent picked and the source, floats the colour,
// and runs the window on; false: not this ask.
func (s *State) applyAskWinTap(k *prep, a Action) bool {
	if s.res.ask != raGenWinTap {
		return false
	}
	r := &s.res
	p := r.askP
	r.ask = raNone
	cost := s.winTapCost(k)
	sl := s.g.winSrc
	col := s.windowTapXCol(k, p, &cost, sl, false, false)
	s.tapXDo(k, p, tapXStep{sl: sl, cand: s.objs[Obj(a.Source())].slot, cand2: NoSlot, col: col})
	if s.payWindow(k, p, &cost) {
		return true
	}
	mode := s.g.winMode
	s.g.winMode = winNone
	if mode == winWard {
		s.finishResolve(k, false)
	} else {
		s.payDone(k, true)
	}
	return true
}
