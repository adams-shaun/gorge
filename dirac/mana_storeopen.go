package dirac

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
