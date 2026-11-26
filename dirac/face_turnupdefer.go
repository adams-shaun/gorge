package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// turnUpDeferSearch: gorge's turn-up is a witnessed potential play (CR 708.6,
// 116.2b), so where the plain census (availFor, which leaves a Taps-deferred
// source out: deferSkip) is short, its prefix search taps the Taps-deferred
// gaps and prices the turn-up from the floats those taps made beside the
// sources left. AkTurnFaceUp is no isScriptPlayKind, so the cast route's
// script (scriptOffers, runPrefixes) never runs for it: this is that route's
// subsets search (deferGapSearch), asked for the turn-up action. A Falls
// under Buried in the Garden is the gap, its tap the ward's watcher.
//
// apply false is the offer's dry probe; true plays the found line's taps, each
// one's triggers flushed below the action as the cast's replay does
// (scriptPrefixAny). The search's own nodes list the turn-up again
// (restrOptionsProbe), so inDeferTU keeps that listing off the search.
//
//go:noinline
func (s *State) turnUpDeferSearch(k *prep, p uint8, a Action, apply bool) bool {
	if k.gen == nil || !k.gen.anyTapsDefer || DebugNoTapsDefer || s.pool != nil && s.pool.inDeferTU != 0 {
		return false
	}
	tapsDef := s.deferGapsForPlay(k, a, s.scriptTapsDefer(k, p))
	if !tapsDef.any() {
		return false
	}
	if s.pool == nil {
		s.pool = new(ForkPool) // (fork makes it the same way)
	}
	s.pool.inDeferTU++
	defer func() { s.pool.inDeferTU-- }()
	var order [MaxPerm]Slot
	n := s.scriptGapOrder(k, a, p, tapsDef, order[:])
	var ans [MaxPerm]uint16
	var pick [MaxPerm]int8
	dead := true
	found := s.deferGapSearch(k, a, p, order[:n], tapsDef, nil, &ans, &pick, &dead)
	if !found || dead {
		s.setAnyColOwedRun(false) // a dry offer taps nothing: the second answer's flag ends with the search
		return false
	}
	if !apply {
		s.setAnyColOwedRun(false)
		return true
	}
	for j := 0; j < n && !s.over; j++ {
		if pick[j] < 0 {
			continue // the found script went on without this gap
		}
		var spill [kop.NumManaKinds]uint8
		s.scriptGapTap(k, p, a, order[j], slotMask{}, slotMask{}, slotMask{}, tapsDef, slotMask{}, int(ans[j]), &spill)
		if s.npend != 0 {
			s.flushPrefixTriggers(k) // each gap's activation is its own decision (CR 603.3b)
			if s.npend != 0 {
				s.resolvePrefixStack(k, s.sp, false)
			}
		}
	}
	s.setAnyColOwedRun(false) // the replay of the search's second run is done
	return true
}

// turnUpDeferPays: the Taps-deferred gaps' search prices the keyword turn-up
// of sl (turnUpDeferSearch).
func (s *State) turnUpDeferPays(k *prep, p uint8, sl Slot, mode uint8) bool {
	return s.turnUpDeferSearch(k, p, MakeAction(AkTurnFaceUp, TCode(s.perms[sl].obj), 0, mode), false)
}

// turnUpDeferOnly: no other route pays the turn-up's cost, only the
// Taps-deferred gaps' search does, so the planner has no plan to tap
// (tapForTurnUp would find none before the gaps' taps).
//
//go:noinline
func (s *State) turnUpDeferOnly(k *prep, p uint8, cost *kop.ManaCost, sp kop.SpecialMask, sl Slot, mode uint8) bool {
	return !s.sourcesPay(k, p, cost) && !s.turnUpRestrPays(k, p, cost, sp, sl) &&
		!s.turnUpPlannerPays(k, p, cost) && !s.turnUpFilterPays(k, p, sl, mode) && !s.turnUpTapXPays(k, p, sl, mode) &&
		s.turnUpDeferPays(k, p, sl, mode)
}

// playTurnUpDefer plays the gaps' search's prefix for the turn-up of o.
func (s *State) playTurnUpDefer(k *prep, p uint8, o Obj, mode uint8) {
	s.turnUpDeferSearch(k, p, MakeAction(AkTurnFaceUp, TCode(o), 0, mode), true)
}
