package dirac

// ctrKey folds into key the counters of the payer's counter-cost sources: gorge
// dedups its prefix search on every own permanent's tapped bit and counters
// (scriptStateKey), so a Vivid Meadow tapped for its plain {W} and one tapped
// for {W} by removing a charge counter are two states, and the second is
// searched. Only a counter-cost mana ability's own counter kind changes along
// the path, and it can change on a source the census covers as well as on a
// gap, so every own permanent with one is folded (bf), not only the gaps.
func (t *State) ctrKey(k *prep, bf slotMask, key *filtKey) {
	if len(k.ctrAb) == 0 {
		return
	}
	i := 0
	for b := bf; b.any(); b = b.rest() {
		sl := b.first()
		a := k.ctrAbOf(t.cardAt(sl))
		if a == nil {
			continue
		}
		key.ctr[i&7] += uint16(t.perms[sl].ctrN(t, int(a.Cost.Ctr)))
		i++
	}
}

// ctrAllKey folds into key every own permanent's +1/+1 and general counters,
// as gorge's scriptStateKey lists each own battlefield object's Counters: a
// trigger the path set off that puts a +1/+1 counter on each creature (Gourmand's
// Talent's level-3 trigger resolving over a Blood Artist trigger the sacrifice
// of a Prism token stacked) is a new payer state beside the stack of the same
// size and pool it left, and the search walks on from it instead of dropping
// it as a visited one. A permanent with no counter folds nothing; the fold is
// a hash, so a counter changing is a different key (kinds in the overflow
// table are not read: no search path moves one).
func (t *State) ctrAllKey(bf slotMask, key *filtKey) {
	const prime = 1099511628211
	var h uint64
	for b := bf; b.any(); b = b.rest() {
		sl := b.first()
		pm := &t.perms[sl]
		if pm.p1 == 0 && pm.ck[0] == 0 {
			continue
		}
		h = (h ^ uint64(sl)) * prime
		h = (h ^ uint64(uint16(pm.p1))) * prime
		for i := range pm.ck {
			if pm.ck[i] == 0 {
				break
			}
			h = (h ^ uint64(pm.ck[i])<<16 ^ uint64(uint16(pm.cn[i]))) * prime
		}
	}
	key.cx = h
}
