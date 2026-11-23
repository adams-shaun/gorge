package dirac

// sacHolderGap: sl is an untapped source whose printed mana ability is
// sacrifice-only (an Eldrazi Spawn's "Sacrifice this creature: Add {C}") and
// that wears a granted {T} mana ability beside it (Jaheira's "{T}: Add {G}"
// on tokens). gorge's census lists the grant as a gap of its own and its
// prefix search activates the holder through the "choose a mana ability" ask:
// the printed sacrifice answer first, then the grant. canTapForMana is false
// for the printed ability (no {T}), so otherGap's tap case never reached the
// holder and the kernel's search never branched on it, finding a funding line
// in a fraction of the nodes gorge's plain run spends before it (CR 605.3b,
// 602.2b).
func (s *State) sacHolderGap(k *prep, sl Slot) bool {
	if k.gen == nil || !k.sacOnlyMana || s.perms[sl].flags&pfTapped != 0 {
		return false
	}
	return s.manaPCGen(k, sl).manaSacOnly && s.canSacOnlyNowAt(k, sl) && s.grantIdx(k, sl) >= 0
}

// sacHolderSpent: sl is a granted-gap holder (sacHolderGap's shape) whose
// grant's tap is spent, so only its printed sacrifice remains. gorge's search
// still takes it in the gap pass, as a one-ability activation with no ask.
func (s *State) sacHolderSpent(k *prep, sl Slot) bool {
	if k.gen == nil || !k.sacOnlyMana || s.perms[sl].flags&pfTapped == 0 {
		return false
	}
	return s.manaPCGen(k, sl).manaSacOnly && s.canSacOnlyNowAt(k, sl) && s.grantIdx(k, sl) >= 0
}
