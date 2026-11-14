package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Taps-deferred gaps in the tap-another search. gorge runs ONE prefix search
// (rules scriptSearch.visit) over every census gap in battlefield order, so a
// depletion land (Peat Bog: its rider makes it a Taps-deferred gap,
// mana_rider.go) beside a real tap-another source (Springleaf Drum) is a gap
// of the same search: the board's prefix belongs to the tap-another search
// (scriptOffers' tx arm, scriptPrefix's tapXPrefix), whose gaps held no
// Taps-deferred source, so a play only the depletion land's mana completes
// was silently dropped. The search now branches on such a gap as the DFS
// search does (scriptDFS: scriptGapTap, deferGapAnswers answers).

// tapXDeferGaps is p's Taps-deferred gaps the tap-another search activates
// beside its gaps: none unless a real tap-another source is modelled (a
// board of counter-cost gaps alone beside a Taps-deferred one is the DFS
// search's, scriptDeadEndAny), and none that is already one of gaps.
func (s *State) tapXDeferGaps(k *prep, p uint8, gaps slotMask) slotMask {
	if !s.scriptTapXModelled(k, p).any() {
		return slotMask{}
	}
	return s.scriptTapsDefer(k, p).andNot(gaps)
}

// deferAsk branches Taps-deferred gap sl at a search node on its activation's
// answers (deferGapAnswers, in the order scriptGapTap decodes: the play's
// announced colours first), the colour ask a node when it has two or more.
// A gap no longer activatable (tapped, out of counters) is skipped. handled:
// the search found a path (found) or ran out of nodes (!found).
func (rs *tapXSearch) deferAsk(k *prep, t *State, sl Slot) (found, handled bool) {
	if !t.scriptTapsDefer(k, rs.p).has(sl) {
		return false, false
	}
	n := t.deferGapAnswers(k, sl)
	if n > 1 && !rs.tick() { // the colour ask
		return false, true
	}
	for c := uint8(0); c < n; c++ {
		if rs.try(k, t, tapXStep{kind: tapXDefer, sl: sl, cand: NoSlot, cand2: NoSlot, col: c, act: rs.a}) {
			return true, true
		}
	}
	return false, false
}

// tapXDeferDo plays Taps-deferred step st: the DFS search's tap of the gap
// for answer st.col (scriptGapTap), priced against the searched play st.act.
func (s *State) tapXDeferDo(k *prep, p uint8, st tapXStep) {
	var spill [kop.NumManaKinds]uint8
	s.scriptGapTap(k, p, st.act, st.sl, slotMask{}, slotMask{}, slotMask{}, slotMask{}.with(st.sl), int(st.col), &spill)
}
