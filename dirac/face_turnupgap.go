package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// turnGapPrefix is turnF's bit 12: the walk of a turn-up's census-gap prefix
// (turnUpGapPrefix) lists only p's Taps-deferred gaps, and stops once the
// planner pays what its pool leaves owed (windowWalkSettled).
const turnGapPrefix turnFT = 1 << 12

// windowOrder lists the sources the window's walk may take: every one p
// controls, or only its Taps-deferred gaps in a turn-up's gap prefix.
func (s *State) windowOrder(k *prep, p uint8, order []Slot) int {
	m := s.ctrlM[p].and(s.occ)
	if s.g.turnF&turnGapPrefix != 0 {
		m = m.and(s.scriptTapsDefer(k, p))
	}
	return s.battlefieldOrder(p, m, order, 0)
}

// windowWalkSettled is windowSpellSettled, or in a turn-up's gap prefix the
// point where the planner's own sources pay what the pool leaves owed.
func (s *State) windowWalkSettled(k *prep, p uint8, cost *kop.ManaCost, anyColor bool) bool {
	return s.windowSpellSettled(k, p, cost, anyColor) ||
		s.g.turnF&turnGapPrefix != 0 && s.turnUpPoolPlanPays(k, p, cost)
}

// turnUpPoolPlanPays: the payment planner pays cost beside p's floating pool
// (a hybrid pip resolved as tapFor resolves it) from its census sources, the
// Taps-deferred gaps left out (planTiered0's first tier). The probe is dry: it
// taps nothing.
func (s *State) turnUpPoolPlanPays(k *prep, p uint8, cost *kop.ManaCost) bool {
	var pk planner
	pk.dry = true
	s.tapForPK(k, p, cost, 0, NoObj, false, false, &pk)
	return pk.needT == 0 || pk.found && !pk.tapsIn // the second tier plans the gaps themselves
}

// turnUpGapPrefix is gorge's census-gap prefix for a keyword-less turn-up
// that only a Taps-deferred any-colour extra (Buried in the Garden, Dawn's
// Reflection) lets the planner's sources pay: the planner's census leaves the
// extra out, so gorge's one prefix search taps the gap lands in battlefield
// order until its planner pays the rest, and the special action's payment
// then plans what the pool leaves owed (key 8: the least creation-rank
// sources), not the window's battlefield order (kfuzz pair 40 of std-mkm). It
// reports whether the prefix was tapped; the caller's planner taps the rest.
// A board whose prefix would not leave the planner a plan keeps the walk of
// tapTurnUpRiders.
//
//go:noinline
func (s *State) turnUpGapPrefix(k *prep, p uint8, cost *kop.ManaCost) bool {
	if k.gen == nil || !s.tapAnyColDeferExtra(k, s.scriptTapsDefer(k, p)) {
		return false
	}
	t := s.fork()
	t.g.turnF |= turnGapPrefix
	t.windowTapWalk(k, p, cost, cost, false, false, false, false, false, false, false)
	ok := t.turnUpPoolPlanPays(k, p, cost)
	t.g.turnF &^= turnGapPrefix
	s.unfork()
	if !ok {
		return false
	}
	s.g.turnF |= turnGapPrefix
	s.windowTapWalk(k, p, cost, cost, false, false, false, false, false, false, false)
	s.g.turnF &^= turnGapPrefix
	return true
}
