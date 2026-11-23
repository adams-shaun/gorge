package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// storeOpenLive reports whether p holds an untapped storage battery (Mercadian
// Bazaar's "{T}, Remove any number of storage counters: Add {R} for each") that
// gorge's PotentialMana walk admits: its cost announces X, so the walk prices the
// activation payable whatever counters it holds and folds its production
// unbounded in every colour (legal.go availPotentialFor's StoreManaAb arm).
// Beside a granted mana ability gorge's walk then lists every play, the
// planner prices it insufficient (the battery is no census unit), and its one
// prefix search over the battery and the grants pays it (CR 601.2g-h,
// 605.1a); the grant search's own walk counts real units only
// (availWalk), so grantCap widens it to the open bound while this holds.
func (s *State) storeOpenLive(k *prep, p uint8) bool {
	for b := s.grantStoreGaps(k, p); b.any(); b = b.rest() {
		sl := b.first()
		c := s.cardAt(sl)
		if s.potentialAdmits(k, p, sl, int(k.ctrAb[c]), k.ctrAbOf(c), nil) {
			return true
		}
	}
	return false
}

// grantPlotCost prices the plot special action (CR 702.170a) for the grant
// search: gorge's walk lists it as a "plot" cast priced at its plot cost, a
// play of its own beside the card's plain cast, so the search funds that cost.
func (s *State) grantPlotCost(k *prep, p uint8, a Action) (cost kop.ManaCost, ex Slot, need [kop.NumManaKinds]bool, ok bool) {
	c := s.objs[a.Source()].card
	return s.adjustCost(k, p, c, Obj(a.Source()), k.t.Cards[c].PlotCost), NoSlot, need, true
}

// grantSuspendCost prices the suspend special action (CR 702.62a) for the
// grant search, grantPlotCost's arm for suspend: gorge's walk lists it as a
// "suspend" cast priced at its suspend cost, a play of its own beside the
// card's plain cast, so the search funds that cost (kw_suspend.go). Without
// it the closed sacrifice-any bound's check (sacPotPays) reads the action
// unpriced and drops a hand card only the bound's tap-another repeats fund
// (tapXRepeatUnits, gorge addPotentialTapRepeat).
func (s *State) grantSuspendCost(k *prep, p uint8, a Action) (cost kop.ManaCost, ex Slot, need [kop.NumManaKinds]bool, ok bool) {
	c := s.objs[a.Source()].card
	return s.adjustCost(k, p, c, Obj(a.Source()), k.t.Cards[c].SuspendCost), NoSlot, need, true
}
