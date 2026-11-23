package dirac

// printedOnlyRetry is filtScript's last fallback for an activation whose
// search spent gorge's whole node budget: the search again with the granted-
// only sources left out of its gaps, walked uncapped and then at each funding
// cap.
//
// gorge prices a printed activation from its payment planner, whose census
// holds printed abilities only (pay/board.go PaymentAbility): a granted tap
// (an Elf under Tyvar Kell's "{T}: Add {B}") is no source it taps, and
// legal_walk_autopay.go's autoPayFast offers the activation without a prefix
// search whenever the printed sources pay it (CR 602.2b, 605.3b). The kernel's
// filter search lists those grants as gaps (otherGaps) and walks their
// activations ahead of the funding pass, so four granted taps beside a filter
// spend the budget on branches the planner never sees and drop an activation
// that Henge of Ramos's "{2},{T}: one mana of any color" and two Islands pay
// (tail-26 pair 66: Niall Silvain's {G}{G}{G}{G},{T} beside three Forests).
// A line found without them is a line the planner pays; a search that runs
// out again proves nothing, and the activation stays unoffered.
func (fs *filtSearch) printedOnlyRetry(s *State, k *prep, t *State) (filtSearch, bool) {
	var granted slotMask
	for m := fs.other; m.any(); m = m.rest() {
		if sl := m.first(); s.manaGranted(k, sl) {
			granted = granted.with(sl)
		}
	}
	if !granted.any() {
		return filtSearch{}, false
	}
	caps := []int{0}
	if fs.usefulOK && fs.useful.MV() > 0 {
		fees := 0
		for m := fs.gaps; m.any(); m = m.rest() {
			fees += s.filtFee(k, m.first())
		}
		for c := fs.useful.MV(); c <= fs.useful.MV()+fees; c++ {
			caps = append(caps, c)
		}
	}
	for _, c := range caps {
		fb := filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
			usefulOK: fs.usefulOK, other: fs.other.andNot(granted), feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead,
			fundCap: c, noHand: fs.noHand, lifePainBound: fs.lifePainBound, fundN: fs.fundN, floated: fs.floated}
		if fb.visit(k, t, 0, false) {
			return fb, true
		}
	}
	return filtSearch{}, false
}
